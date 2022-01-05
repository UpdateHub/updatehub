// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{Context, Error, Result};
use crate::{
    object::{Info, Installer},
    utils::{self, definitions::TargetTypeExt, log::LogContent},
};
use pkg_schema::{definitions, objects};
use slog_scope::info;
use std::os::unix::fs::PermissionsExt;
use tokio::{
    fs,
    io::{self, AsyncWriteExt},
};

#[async_trait::async_trait(?Send)]
impl Installer for objects::Copy {
    async fn check_requirements(&self, _: &Context) -> Result<()> {
        info!("'copy' handle checking requirements");

        if let definitions::TargetType::Device(dev) =
            self.target_type.valid().log_error_msg("device failed vaidation")?
        {
            utils::fs::ensure_disk_space(dev, self.required_install_size())
                .log_error_msg("not enough disk space")?;
            return Ok(());
        }

        Err(Error::InvalidTargetType(self.target_type.clone()))
    }

    async fn install(&self, context: &Context) -> Result<()> {
        info!("'copy' handler Install {} ({})", self.filename, self.sha256sum);

        let device = self.target_type.get_target().log_error_msg("failed to get target device")?;
        let filesystem = self.filesystem;
        let mount_options = &self.mount_options;
        let format_options = &self.target_format.format_options;
        let chunk_size = definitions::ChunkSize::default().0;
        let sha256sum = self.sha256sum();
        let target_path = self.target_path.strip_prefix("/").unwrap_or(&self.target_path);
        let source = context.download_dir.join(sha256sum);

        let should_skip_install =
            super::should_skip_install(&self.install_if_different, &self.sha256sum, async {
                utils::fs::mount_map_async(&device, filesystem, mount_options, |path| async move {
                    fs::File::open(&path.join(&target_path)).await.map_err(Error::from)
                })
                .await
                .map_err(Error::from)
                .and_then(|r| r)
            })
            .await?;
        if should_skip_install {
            return Ok(());
        }

        if self.target_format.should_format {
            utils::fs::format(&device, filesystem, format_options)
                .log_error_msg("failed to format partition")?;
        }

        utils::fs::mount_map_async(&device, filesystem, mount_options, |path| async move {
            let dest = path.join(&target_path);
            let mut input = utils::io::timed_buf_reader(
                chunk_size,
                fs::File::open(source).await.log_error_msg("failed to open source object")?,
            );
            let mut output = utils::io::timed_buf_writer(
                chunk_size,
                fs::OpenOptions::new()
                    .read(true)
                    .write(true)
                    .create(true)
                    .truncate(true)
                    .open(&dest)
                    .await
                    .log_error_msg("failed to open target file")?,
            );

            // File's access mode is changed here as we might not have write permission over
            // it. It will be restored or overwritten later on by the target_mode parameter
            let metadata = dest.metadata().log_error_msg("failed to get target metadata")?;
            let orig_mode = metadata.permissions().mode();
            metadata.permissions().set_mode(0o100_666);

            if self.compressed {
                compress_tools::tokio_support::uncompress_data(&mut input, &mut output)
                    .await
                    .log_error_msg("failed to uncompress data")?;
            } else {
                io::copy(&mut input, &mut output)
                    .await
                    .log_error_msg("failed to copy from object to target")?;
            }
            output.flush().await.log_error_msg("failed to flush disk write")?;
            metadata.permissions().set_mode(orig_mode);

            if let Some(mode) = self.target_permissions.target_mode {
                utils::fs::chmod(&dest, mode).log_error_msg("failed to update permission")?;
            }

            utils::fs::chown(
                &dest,
                &self.target_permissions.target_uid,
                &self.target_permissions.target_gid,
            )
            .log_error_msg("failed to update ownership")?;

            Ok(())
        })
        .await
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
        io::{Seek, SeekFrom, Write},
        iter,
        os::unix::fs::MetadataExt,
        path::PathBuf,
    };
    use tokio::io::AsyncBufReadExt;

    const DEFAULT_BYTE: u8 = 0xF;
    const ORIGINAL_BYTE: u8 = 0xA;
    const FILE_SIZE: usize = 2048;

    async fn exec_test_with_copy<F>(
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
            utils::fs::mount_map_async(
                &device,
                definitions::Filesystem::Ext4,
                "",
                |path| async move {
                    let file = path.join(&"original_file");
                    fs::File::create(&file)
                        .await?
                        .write_all(&iter::repeat(ORIGINAL_BYTE).take(FILE_SIZE).collect::<Vec<_>>())
                        .await?;

                    if let Some(mode) = perm.target_mode {
                        utils::fs::chmod(&file, mode)?;
                    }

                    utils::fs::chown(&file, &perm.target_uid, &perm.target_gid)?;

                    utils::Result::Ok(())
                },
            )
            .await??;
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
        obj.check_requirements(&Context::default()).await?;
        obj.install(&Context::default()).await?;

        // Validade File
        #[allow(clippy::redundant_clone)]
        utils::fs::mount_map_async(
            &device,
            obj.filesystem,
            &obj.mount_options.clone(),
            |path| async move {
                let chunk_size = definitions::ChunkSize::default().0;
                let dest = path.join(&obj.target_path);
                let mut rd1 = io::BufReader::with_capacity(chunk_size, original_data.as_slice());
                let mut rd2 =
                    io::BufReader::with_capacity(chunk_size, fs::File::open(&dest).await?);

                loop {
                    let buf1 = rd1.fill_buf().await?;
                    let len1 = buf1.len();
                    let buf2 = rd2.fill_buf().await?;
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
            },
        )
        .await??;

        loopdev.detach()?;

        Ok(())
    }

    #[tokio::test]
    #[ignore]
    async fn copy_compressed_file() {
        exec_test_with_copy(|obj| obj.compressed = true, None, true).await.unwrap();
    }

    #[tokio::test]
    #[ignore]
    async fn copy_over_formated_partion() {
        exec_test_with_copy(|obj| obj.target_format.should_format = true, None, false)
            .await
            .unwrap();
    }

    #[tokio::test]
    #[ignore]
    async fn copy_over_existing_file() {
        exec_test_with_copy(
            |_| (),
            Some(definitions::TargetPermissions {
                target_mode: Some(0o666),
                target_gid: Some(definitions::target_permissions::Gid::Number(1000)),
                target_uid: Some(definitions::target_permissions::Uid::Number(1000)),
            }),
            false,
        )
        .await
        .unwrap();
    }

    #[tokio::test]
    #[ignore]
    async fn copy_change_uid() {
        exec_test_with_copy(
            |obj| {
                obj.target_permissions.target_uid =
                    Some(definitions::target_permissions::Uid::Number(0))
            },
            None,
            false,
        )
        .await
        .unwrap();
    }

    #[tokio::test]
    #[ignore]
    async fn copy_change_gid() {
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
        .await
        .unwrap();
    }

    #[tokio::test]
    #[ignore]
    async fn copy_change_mode() {
        exec_test_with_copy(
            |obj| obj.target_permissions.target_mode = Some(0o444),
            Some(definitions::TargetPermissions {
                target_mode: Some(0o666),
                target_gid: Some(definitions::target_permissions::Gid::Number(1000)),
                target_uid: Some(definitions::target_permissions::Uid::Number(1000)),
            }),
            false,
        )
        .await
        .unwrap();
    }
}
