// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{Context, Result};
use crate::{
    object::{Info, Installer},
    utils::{self, definitions::TargetTypeExt, log::LogContent},
};
use pkg_schema::{definitions, objects};
use slog_scope::info;

#[async_trait::async_trait(?Send)]
impl Installer for objects::Tarball {
    async fn check_requirements(&self, _: &Context) -> Result<()> {
        info!("'tarball' handle checking requirements");

        match self.target {
            definitions::TargetType::Device(_)
            | definitions::TargetType::UBIVolume(_)
            | definitions::TargetType::MTDName(_) => {
                utils::fs::ensure_disk_space(
                    &self.target.get_target().log_error_msg("failed to get target device")?,
                    self.required_install_size(),
                )
                .log_error_msg("not enough disk space")?;
                self.target.valid().log_error_msg("device failed validation")?;
                Ok(())
            }
        }
    }

    async fn install(&self, context: &Context) -> Result<()> {
        info!("'tarball' handler Install {} ({})", self.filename, self.sha256sum);

        let device = self.target.get_target().log_error_msg("failed to get target device")?;
        let filesystem = self.filesystem;
        let mount_options = &self.mount_options;
        let format_options = &self.target_format.format_options;
        let sha256sum = self.sha256sum();
        let target_path = self.target_path.strip_prefix("/").unwrap_or(&self.target_path);
        let source = context.download_dir.join(sha256sum);

        if self.target_format.should_format {
            utils::fs::format(&device, filesystem, format_options)
                .log_error_msg("failed to format partition")?;
        }

        let mount_guard = utils::fs::mount(&device, filesystem, mount_options)?;
        let dest = mount_guard.mount_point().join(target_path);
        let mut source =
            tokio::fs::File::open(source).await.log_error_msg("failed to open source object")?;
        compress_tools::tokio_support::uncompress_archive(
            &mut source,
            &dest,
            compress_tools::Ownership::Preserve,
        )
        .await
        .log_error_msg("failed to uncompress tar object to target")?;
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::object::installer::tests::SERIALIZE;
    use pretty_assertions::assert_eq;
    use std::{
        io::{Seek, SeekFrom, Write},
        os::unix::fs::MetadataExt,
        path::{Path, PathBuf},
    };

    const CONTENT_SIZE: usize = 10240;

    async fn exec_test_with_tarball<F>(mut f: F) -> Result<()>
    where
        F: FnMut(&mut objects::Tarball),
    {
        // Generate a sparse file for the faked device use
        let mut image = tempfile::NamedTempFile::new()?;
        image.seek(SeekFrom::Start(1024 * 1024 + CONTENT_SIZE as u64))?;
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

        // Generate base copy object
        let mut obj = objects::Tarball {
            filename: "".to_string(),
            filesystem: definitions::Filesystem::Ext4,
            size: CONTENT_SIZE as u64,
            sha256sum: "tree.tar".to_string(),
            target: definitions::TargetType::Device(device.clone()),
            target_path: PathBuf::from("/"),

            compressed: false,
            required_uncompressed_size: CONTENT_SIZE as u64,
            target_format: definitions::TargetFormat::default(),
            mount_options: String::default(),
        };
        f(&mut obj);
        let context = Context { download_dir: PathBuf::from("fixtures"), ..Context::default() };

        // Setup preinstall structure
        {
            let mount_guard = utils::fs::mount(&device, definitions::Filesystem::Ext4, "")?;
            tokio::fs::create_dir(mount_guard.mount_point().join("existing_dir")).await?;
        }

        // Peform Install
        obj.check_requirements(&context).await?;
        obj.install(&context).await?;

        // Validade File
        {
            let mount_guard = utils::fs::mount(&device, obj.filesystem, &obj.mount_options)?;
            let assert_metadata = |p: &Path| -> crate::utils::Result<()> {
                let metadata = p.metadata()?;
                assert_eq!(metadata.mode() % 0o1000, 0o664);
                assert_eq!(metadata.uid(), 1000);
                assert_eq!(metadata.gid(), 1000);

                Ok(())
            };
            let dest = mount_guard
                .mount_point()
                .join(&obj.target_path.strip_prefix("/").map_err(utils::Error::from)?);
            assert_metadata(&dest.join("tree/branch1/leaf"))?;
            assert_metadata(&dest.join("tree/branch2/leaf"))?;
        }

        loopdev.detach()?;

        Ok(())
    }

    #[tokio::test]
    #[ignore]
    async fn install_over_formated_partion() {
        exec_test_with_tarball(|obj| obj.target_format.should_format = true).await.unwrap();
    }

    #[tokio::test]
    #[ignore]
    async fn install_over_unformated_partion() {
        exec_test_with_tarball(|obj| obj.target_path = PathBuf::from("/existing_dir"))
            .await
            .unwrap();
    }
}
