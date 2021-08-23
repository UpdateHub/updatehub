// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{Context, Error, Result};
use crate::{
    object::{Info, Installer},
    utils::{self, definitions::TargetTypeExt},
};
use pkg_schema::{definitions, objects};
use slog_scope::info;
use std::{
    fs,
    io::{self, Write},
    os::unix::fs::PermissionsExt,
};

impl Installer for objects::Copy {
    fn check_requirements(&self, _: &Context) -> Result<()> {
        info!("'copy' handle checking requirements");

        if let definitions::TargetType::Device(dev) = self.target_type.valid()? {
            utils::fs::ensure_disk_space(dev, self.required_install_size())?;
            return Ok(());
        }

        Err(Error::InvalidTargetType(self.target_type.clone()))
    }

    fn install(&self, context: &Context) -> Result<()> {
        info!("'copy' handler Install {} ({})", self.filename, self.sha256sum);

        let device = self.target_type.get_target()?;
        let filesystem = self.filesystem;
        let mount_options = &self.mount_options;
        let format_options = &self.target_format.format_options;
        let chunk_size = definitions::ChunkSize::default().0;
        let sha256sum = self.sha256sum();
        let target_path = self.target_path.strip_prefix("/").unwrap_or(&self.target_path);
        let source = context.download_dir.join(sha256sum);

        handle_install_if_different!(self.install_if_different, sha256sum, {
            utils::fs::mount_map(&device, filesystem, mount_options, |path| {
                fs::File::open(&path.join(&target_path)).map_err(Error::from)
            })
            .map_err(Error::from)
            .and_then(|r| r)
        });

        if self.target_format.should_format {
            utils::fs::format(&device, filesystem, format_options)?;
        }

        utils::fs::mount_map(&device, filesystem, mount_options, |path| {
            let dest = path.join(&target_path);
            let mut input = utils::io::timed_buf_reader(chunk_size, fs::File::open(source)?);
            let mut output = utils::io::timed_buf_writer(
                chunk_size,
                fs::OpenOptions::new()
                    .read(true)
                    .write(true)
                    .create(true)
                    .truncate(true)
                    .open(&dest)?,
            );

            // File's access mode is changed here as we might not have write permission over
            // it. It will be restored or overwritten later on by the target_mode parameter
            let metadata = dest.metadata()?;
            let orig_mode = metadata.permissions().mode();
            metadata.permissions().set_mode(0o100_666);

            if self.compressed {
                compress_tools::uncompress_data(&mut input, &mut output)?;
            } else {
                io::copy(&mut input, &mut output)?;
            }
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
        .map_err(Error::from)
        .and_then(|r| r)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::{object::installer::tests::SERIALIZE, utils::definitions::IdExt};
    use flate2::{write::GzEncoder, Compression};
    use pretty_assertions::assert_eq;
    use std::{
        io::{BufRead, Seek, SeekFrom, Write},
        iter,
        os::unix::fs::MetadataExt,
        path::PathBuf,
    };

    const DEFAULT_BYTE: u8 = 0xF;
    const ORIGINAL_BYTE: u8 = 0xA;
    const FILE_SIZE: usize = 2048;

    fn exec_test_with_copy<F>(
        mut f: F,
        original_permissions: Option<definitions::TargetPermissions>,
        compressed: bool,
    ) -> Result<()>
    where
        F: FnMut(&mut objects::Copy),
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
        let original_data = iter::repeat(DEFAULT_BYTE).take(FILE_SIZE).collect::<Vec<_>>();
        let data = if compressed {
            let mut e = GzEncoder::new(Vec::new(), Compression::default());
            e.write_all(&original_data).unwrap();
            e.finish().unwrap()
        } else {
            original_data.clone()
        };
        source.write_all(&data)?;

        // When needed, create a file inside the mounted device
        if let Some(perm) = original_permissions {
            utils::fs::mount_map(&device, definitions::Filesystem::Ext4, "", |path| {
                let file = path.join(&"original_file");
                fs::File::create(&file)?
                    .write_all(&iter::repeat(ORIGINAL_BYTE).take(FILE_SIZE).collect::<Vec<_>>())?;

                if let Some(mode) = perm.target_mode {
                    utils::fs::chmod(&file, mode)?;
                }

                utils::fs::chown(&file, &perm.target_uid, &perm.target_gid)?;

                utils::Result::Ok(())
            })??;
        }

        // Generate base copy object
        let mut obj = objects::Copy {
            filename: "".to_string(),
            filesystem: definitions::Filesystem::Ext4,
            size: FILE_SIZE as u64,
            sha256sum: source.path().to_string_lossy().to_string(),
            target_type: definitions::TargetType::Device(device.clone()),
            target_path: PathBuf::from("original_file"),
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
        obj.check_requirements(&Context::default())?;
        obj.install(&Context::default())?;

        // Validade File
        #[allow(clippy::redundant_clone)]
        utils::fs::mount_map(&device, obj.filesystem, &obj.mount_options.clone(), |path| {
            let chunk_size = definitions::ChunkSize::default().0;
            let dest = path.join(&obj.target_path);
            let mut rd1 = io::BufReader::with_capacity(chunk_size, original_data.as_slice());
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

            std::io::Result::Ok(())
        })??;

        loopdev.detach()?;

        Ok(())
    }

    #[test]
    #[ignore]
    fn copy_compressed_file() {
        exec_test_with_copy(|obj| obj.compressed = true, None, true).unwrap();
    }

    #[test]
    #[ignore]
    fn copy_over_formated_partion() {
        exec_test_with_copy(|obj| obj.target_format.should_format = true, None, false).unwrap();
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
            false,
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
            false,
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
            false,
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
            false,
        )
        .unwrap();
    }
}
