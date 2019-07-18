// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::{
    object::{Info, Installer},
    utils,
};
use pkg_schema::{definitions, objects};
use slog::slog_info;
use slog_scope::info;
use std::path::Path;

impl Installer for objects::Tarball {
    fn check_requirements(&self) -> Result<(), failure::Error> {
        info!("'tarball' handle checking requirements");

        match self.target {
            definitions::TargetType::Device(_)
            | definitions::TargetType::UBIVolume(_)
            | definitions::TargetType::MTDName(_) => self.target.valid().map(|_| ()),
        }
    }

    fn install(&self, download_dir: &Path) -> Result<(), failure::Error> {
        info!("'tarball' handler Install");

        let device = self.target.get_target()?;
        let filesystem = self.filesystem;
        let mount_options = &self.mount_options;
        let format_options = &self.target_format.format_options;
        let source = download_dir.join(self.sha256sum());

        // FIXME: use required_uncompressed_size
        // if we will format, we check the full size
        // else we check the remaning size

        if self.target_format.should_format {
            utils::fs::format(&device, filesystem, format_options)?;
        }

        utils::fs::mount_map(&device, filesystem, mount_options, |path| {
            let dest = path.join(&self.target_path.strip_prefix("/")?);

            compress_tools::uncompress(
                &source,
                &dest,
                utils::fs::find_compress_tarball_kind(&source)?,
            )
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use lazy_static::lazy_static;
    use loopdev;
    use pretty_assertions::assert_eq;
    use std::{
        fs,
        io::{Seek, SeekFrom, Write},
        os::unix::fs::MetadataExt,
        path::{Path, PathBuf},
        sync::{Arc, Mutex},
    };
    use tempfile;

    const CONTENT_SIZE: usize = 10240;

    lazy_static! {
        static ref SERIALIZE: Arc<Mutex<()>> = Arc::new(Mutex::default());
    }

    fn exec_test_with_tarball<F>(mut f: F) -> Result<(), failure::Error>
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

        // Setup preinstall structure
        utils::fs::mount_map(&device, definitions::Filesystem::Ext4, &"", |path| {
            fs::create_dir(path.join("existing_dir"))?;
            Ok(())
        })?;

        // Peform Install
        obj.check_requirements()?;
        obj.setup()?;
        obj.install(&PathBuf::from("test/fixtures"))?;

        // Validade File
        utils::fs::mount_map(
            &device,
            obj.filesystem,
            &obj.mount_options.clone(),
            |path| {
                let assert_metadata = |p: &Path| -> Result<(), failure::Error> {
                    let metadata = p.metadata()?;
                    assert_eq!(metadata.mode() % 0o1000, 0o664);
                    assert_eq!(metadata.uid(), 1000);
                    assert_eq!(metadata.gid(), 1000);

                    Ok(())
                };
                let dest = path.join(&obj.target_path.strip_prefix("/")?);
                assert_metadata(&dest.join("tree/branch1/leaf"))?;
                assert_metadata(&dest.join("tree/branch2/leaf"))?;

                Ok(())
            },
        )?;

        loopdev.detach()?;

        Ok(())
    }

    #[test]
    #[ignore]
    fn install_over_formated_partion() {
        exec_test_with_tarball(|obj| obj.target_format.should_format = true).unwrap();
    }

    #[test]
    #[ignore]
    fn install_over_unformated_partion() {
        exec_test_with_tarball(|obj| obj.target_path = PathBuf::from("/existing_dir")).unwrap();
    }
}
