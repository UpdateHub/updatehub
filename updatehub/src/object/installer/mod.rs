// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

mod copy;
mod flash;
mod imxkobs;
mod mender;
mod raw;
mod tarball;
mod test;
mod ubifs;
mod uboot_env;
mod zephyr;

use super::{Error, Result};
use crate::utils;
use find_binary_version::{self as fbv, BinaryKind};
use pkg_schema::{definitions, Object};
use slog_scope::debug;
use std::io;

pub(crate) trait Installer {
    fn check_requirements(&self) -> Result<()> {
        debug!("running default check_requirements");
        Ok(())
    }

    fn install(&self, download_dir: &std::path::Path) -> Result<()>;
}

impl Installer for Object {
    fn check_requirements(&self) -> Result<()> {
        for_any_object!(self, o, { o.check_requirements() })
    }

    fn install(&self, download_dir: &std::path::Path) -> Result<()> {
        for_any_object!(self, o, { o.install(download_dir) })
    }
}

fn check_if_different<R: io::Read + io::Seek>(
    handle: &mut R,
    rule: &definitions::InstallIfDifferent,
    sha256sum: &str,
) -> Result<bool> {
    match rule {
        definitions::InstallIfDifferent::CheckSum => {
            let mut buffer = Vec::default();
            handle.read_to_end(&mut buffer)?;
            if utils::sha256sum(&buffer) == sha256sum {
                return Ok(true);
            }
        }
        definitions::InstallIfDifferent::KnownPattern { version, pattern } => {
            let pattern = match pattern {
                definitions::install_if_different::KnownPatternKind::UBoot => BinaryKind::UBoot,
                definitions::install_if_different::KnownPatternKind::LinuxKernel => {
                    BinaryKind::LinuxKernel
                }
            };
            if let Some(ref cur_version) = fbv::version(handle, pattern) {
                if version == cur_version {
                    return Ok(true);
                }
            }
        }
        definitions::InstallIfDifferent::CustomPattern { version, pattern } => {
            io::Seek::seek(handle, io::SeekFrom::Start(pattern.seek))?;
            let mut src = io::BufReader::with_capacity(pattern.buffer_size as usize, handle);
            if let Some(ref cur_version) = fbv::version_with_pattern(&mut src, &pattern.regexp) {
                if version == cur_version {
                    return Ok(true);
                }
            }
        }
    }
    Ok(false)
}

#[cfg(test)]
mod tests {
    use super::*;
    use lazy_static::lazy_static;
    use std::{
        env, fs,
        io::Write,
        os::unix::fs::PermissionsExt,
        path::{Path, PathBuf},
        sync::{Arc, Mutex},
    };
    use tempfile::TempDir;

    // Used to serialize access to Loop devices across tests
    lazy_static! {
        pub static ref SERIALIZE: Arc<Mutex<()>> = Arc::new(Mutex::default());
    }

    fn create_echo_bin(bin: &Path, output: &Path) -> std::io::Result<()> {
        let mut file = std::fs::File::create(bin)?;
        file.write_all(
            format!(
                "#!/bin/sh\necho {} $@ >> {:?}\n",
                bin.file_name().unwrap().to_str().unwrap(),
                output
            )
            .as_bytes(),
        )?;
        file.set_permissions(fs::Permissions::from_mode(0o777))?;

        Ok(())
    }

    pub fn create_echo_bins(bins: &[&str]) -> std::io::Result<(TempDir, PathBuf)> {
        let mocks = tempfile::tempdir()?;
        let mocks_dir = mocks.path();
        let calls = mocks_dir.join("calls");

        for bin in bins {
            create_echo_bin(&mocks_dir.join(bin), &calls)?;
        }

        env::set_var(
            "PATH",
            format!(
                "{}{}",
                mocks_dir.display(),
                &env::var("PATH").map(|s| format!(":{}", s)).unwrap_or_default()
            ),
        );

        Ok((mocks, calls))
    }

    #[test]
    fn unmatched_checksum() {
        let mut f = tempfile::NamedTempFile::new().unwrap();
        assert_eq!(
            check_if_different(
                &mut f,
                &definitions::InstallIfDifferent::CheckSum,
                "some_sha256sum"
            )
            .unwrap(),
            false,
            "Empty fille should not be validated to the checksum"
        );
    }

    #[test]
    fn checksum() {
        let mut f = tempfile::NamedTempFile::new().unwrap();
        io::Write::write_all(&mut f, b"some_sha256sum").unwrap();
        assert_eq!(
            check_if_different(
                &mut f,
                &definitions::InstallIfDifferent::CheckSum,
                "7dc201ce54a835790d78835363a0bce4db704dd23c0c05e399d2a7d1f8fcef19",
            )
            .unwrap(),
            false,
            "Empty fille should not be validated to the checksum"
        );
    }
}
