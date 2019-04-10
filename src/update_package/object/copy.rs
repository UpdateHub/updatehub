// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{definitions, ObjectInstaller, ObjectType};
use crate::utils;
use failure::bail;
use serde::Deserialize;
use slog::slog_info;
use slog_scope::info;
use std::{
    fs,
    io::{self, Write},
    os::unix::fs::PermissionsExt,
    path::PathBuf,
};

#[derive(Deserialize, PartialEq, Debug)]
#[serde(rename_all = "kebab-case")]
pub(crate) struct Copy {
    filename: String,
    filesystem: definitions::Filesystem,
    size: u64,
    sha256sum: String,
    #[serde(flatten)]
    target_type: definitions::TargetType,
    target_path: String,

    install_if_different: Option<definitions::InstallIfDifferent>,
    #[serde(flatten)]
    target_permissions: definitions::TargetPermissions,
    #[serde(default)]
    compressed: bool,
    #[serde(default)]
    required_uncompressed_size: u64,
    #[serde(flatten, default)]
    target_format: definitions::TargetFormat,
    #[serde(default)]
    mount_options: String,
}

impl_object_type!(Copy);

impl ObjectInstaller for Copy {
    fn check_requirements(&self) -> Result<(), failure::Error> {
        info!("'copy' handle checking requirements");
        if let definitions::TargetType::Device(_) = self.target_type.valid()? {
            return Ok(());
        }

        bail!("Unexpected target type, expected some device.")
    }

    fn install(&self, download_dir: PathBuf) -> Result<(), failure::Error> {
        info!("'copy' handler Install");

        let device = match self.target_type {
            definitions::TargetType::Device(ref p) => p,
            _ => unreachable!("Device should be secured by check_requirements"),
        };

        let filesystem = self.filesystem;
        let mount_options = &self.mount_options;
        let format_options = &self.target_format.format_options;
        let chunk_size = definitions::ChunkSize::default().0;

        if self.target_format.format {
            utils::fs::format(device, filesystem, &format_options)?;
        }

        utils::fs::mount_map(device, filesystem, mount_options, |path| {
            let dest = path.join(&self.target_path);
            let source = download_dir.join(self.sha256sum());
            let mut input = io::BufReader::with_capacity(chunk_size, fs::File::open(source)?);
            let mut output = io::BufWriter::with_capacity(
                chunk_size,
                fs::OpenOptions::new()
                    .read(true)
                    .write(true)
                    .create(true)
                    .truncate(true)
                    .open(&dest)?,
            );

            let metadata = dest.metadata()?;
            let orig_mode = metadata.permissions().mode();
            metadata.permissions().set_mode(0o100_666);
            io::copy(&mut input, &mut output)?;
            output.flush()?;
            metadata.permissions().set_mode(orig_mode);

            if let Some(mode) = self.target_permissions.target_mode {
                utils::fs::chmod(&dest, mode)?;
            }

            utils::fs::chown(
                &dest,
                &self.target_permissions.target_uid,
                &self.target_permissions.target_gid,
            )?;

            Ok(())
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use lazy_static::lazy_static;
    use pretty_assertions::assert_eq;
    use serde_json::json;
    use std::{
        io::{BufRead, Seek, SeekFrom, Write},
        iter,
        os::unix::fs::MetadataExt,
        path::PathBuf,
        sync::{Arc, Mutex},
    };

    lazy_static! {
        static ref SERIALIZE: Arc<Mutex<()>> = Arc::new(Mutex::default());
    }

    const DEFAULT_BYTE: u8 = 0xF;
    const ORIGINAL_BYTE: u8 = 0xA;
    const FILE_SIZE: usize = 2048;

    fn exec_test_with_copy<F>(
        mut f: F,
        original_permissions: Option<definitions::TargetPermissions>,
    ) -> Result<(), failure::Error>
    where
        F: FnMut(&mut Copy),
    {
        // Generate a sparse file for the faked device use
        let mut image = tempfile::NamedTempFile::new()?;
        image.seek(SeekFrom::Start(1024 * 1024 + FILE_SIZE as u64))?;
        image.write_all(&[0])?;

        // Setup faked device
        let (loopdev, device) = {
            // Loop device next_free is not thread safe
            let mutex = SERIALIZE.clone();
            let _mutex = mutex.lock().unwrap();
            let loopdev = loopdev::LoopControl::open()?.next_free()?;
            let device = loopdev.path().unwrap();
            loopdev.attach_file(image.path())?;
            (loopdev, device)
        };

        // Format the faked device
        utils::fs::format(&device, definitions::Filesystem::Ext4, &None)?;

        // Generate the source file
        let download_dir = tempfile::tempdir()?;
        let mut source = tempfile::NamedTempFile::new_in(download_dir.path())?;
        source.write_all(
            &iter::repeat(DEFAULT_BYTE)
                .take(FILE_SIZE)
                .collect::<Vec<_>>(),
        )?;

        // When needed, create a file inside the mounted device
        if let Some(perm) = original_permissions {
            utils::fs::mount_map(&device, definitions::Filesystem::Ext4, &"", |path| {
                let file = path.join(&"original_file");
                fs::File::create(&file)?.write_all(
                    &iter::repeat(ORIGINAL_BYTE)
                        .take(FILE_SIZE)
                        .collect::<Vec<_>>(),
                )?;

                if let Some(mode) = perm.target_mode {
                    utils::fs::chmod(&file, mode)?;
                }

                utils::fs::chown(&file, &perm.target_uid, &perm.target_gid)?;

                Ok(())
            })?;
        }

        // Generate base copy object
        let mut obj = Copy {
            filename: "".to_string(),
            filesystem: definitions::Filesystem::Ext4,
            size: FILE_SIZE as u64,
            sha256sum: source.path().to_string_lossy().to_string(),
            target_type: definitions::TargetType::Device(device.clone()),
            target_path: "original_file".to_string(),
            install_if_different: None,
            target_permissions: definitions::TargetPermissions::default(),
            compressed: false,
            required_uncompressed_size: 0,
            target_format: definitions::TargetFormat::default(),
            mount_options: String::default(),
        };

        // Change copy object to be used on current test
        f(&mut obj);

        // Peform Install
        obj.check_requirements()?;
        obj.setup()?;
        obj.install(download_dir.path().to_path_buf())?;

        // Validade File
        utils::fs::mount_map(
            &device,
            obj.filesystem,
            &obj.mount_options.clone(),
            |path| {
                let chunk_size = definitions::ChunkSize::default().0;
                let dest = path.join(&obj.target_path);
                let source = download_dir.path().join(&obj.sha256sum);
                let mut rd1 = io::BufReader::with_capacity(chunk_size, fs::File::open(&source)?);
                let mut rd2 = io::BufReader::with_capacity(chunk_size, fs::File::open(&dest)?);

                loop {
                    let buf1 = rd1.fill_buf()?;
                    let len1 = buf1.len();
                    let buf2 = rd2.fill_buf()?;
                    let len2 = buf2.len();
                    // Stop comparing when both the files reach EOF
                    if len1 == 0 && len2 == 0 {
                        break;
                    }
                    assert_eq!(buf1, buf2);
                    rd1.consume(len1);
                    rd2.consume(len2);
                }

                let metadata = dest.metadata()?;
                if let Some(mode) = obj.target_permissions.target_mode {
                    assert_eq!(mode, metadata.mode() % 0o1000);
                };

                if let Some(uid) = obj.target_permissions.target_uid {
                    let uid = uid.as_u32();
                    assert_eq!(uid, metadata.uid());
                };

                if let Some(gid) = obj.target_permissions.target_gid {
                    let gid = gid.as_u32();
                    assert_eq!(gid, metadata.gid());
                };

                Ok(())
            },
        )?;

        loopdev.detach()?;

        Ok(())
    }

    #[test]
    #[ignore]
    fn copy_over_formated_partion() {
        exec_test_with_copy(|obj| obj.target_format.format = true, None).unwrap();
    }

    #[test]
    #[ignore]
    fn copy_over_existing_file() {
        exec_test_with_copy(
            |_| (),
            Some(definitions::TargetPermissions {
                target_mode: Some(0o666),
                target_gid: Some(definitions::target_permissions::Gid::Number(1000)),
                target_uid: Some(definitions::target_permissions::Uid::Number(1000)),
            }),
        )
        .unwrap();
    }

    #[test]
    #[ignore]
    fn copy_change_uid() {
        exec_test_with_copy(
            |obj| {
                obj.target_permissions.target_uid =
                    Some(definitions::target_permissions::Uid::Number(0))
            },
            None,
        )
        .unwrap();
    }

    #[test]
    #[ignore]
    fn copy_change_gid() {
        exec_test_with_copy(
            |obj| {
                obj.target_permissions.target_gid =
                    Some(definitions::target_permissions::Gid::Number(0))
            },
            Some(definitions::TargetPermissions {
                target_mode: Some(0o666),
                target_gid: Some(definitions::target_permissions::Gid::Number(1000)),
                target_uid: Some(definitions::target_permissions::Uid::Number(1000)),
            }),
        )
        .unwrap();
    }

    #[test]
    #[ignore]
    fn copy_change_mode() {
        exec_test_with_copy(
            |obj| obj.target_permissions.target_mode = Some(0o444),
            Some(definitions::TargetPermissions {
                target_mode: Some(0o666),
                target_gid: Some(definitions::target_permissions::Gid::Number(1000)),
                target_uid: Some(definitions::target_permissions::Uid::Number(1000)),
            }),
        )
        .unwrap();
    }

    #[test]
    fn deserialize() {
        assert_eq!(
            Copy {
                filename: "etc/passwd".to_string(),
                filesystem: definitions::Filesystem::Btrfs,
                size: 1024,
                sha256sum: "cfe2be1c64b0387500853de0f48303e3de7b1c6f1508dc719eeafa0d41c36722"
                    .to_string(),
                target_type: definitions::TargetType::Device(PathBuf::from("/dev/sda")),
                target_path: "/etc/passwd".to_string(),

                install_if_different: Some(definitions::InstallIfDifferent::CheckSum(
                    definitions::install_if_different::CheckSum::Sha256Sum
                )),
                target_permissions: definitions::TargetPermissions::default(),
                compressed: false,
                required_uncompressed_size: 0,
                target_format: definitions::TargetFormat::default(),
                mount_options: String::default(),
            },
            serde_json::from_value::<Copy>(json!({
                "filename": "etc/passwd",
                "size": 1024,
                "sha256sum": "cfe2be1c64b0387500853de0f48303e3de7b1c6f1508dc719eeafa0d41c36722",
                "install-if-different": "sha256sum",
                "filesystem": "btrfs",
                "target-type": "device",
                "target": "/dev/sda",
                "target-path": "/etc/passwd"
            }))
            .unwrap()
        );
    }
}
