// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

mod copy;
mod flash;
mod imxkobs;
mod mender;
mod raw;
mod raw_delta;
mod tarball;
mod test;
mod ubifs;
mod uboot_env;
mod zephyr;

use super::{Error, Result};
use crate::utils;
use find_binary_version::{self as fbv, BinaryKind};
use pkg_schema::{Object, definitions};
use slog_scope::{debug, error, info, trace};
use std::{io, path::PathBuf};
use tokio::io::{AsyncRead, AsyncReadExt, AsyncSeek, AsyncSeekExt, BufReader};

#[derive(Clone, Debug, Default)]
pub(crate) struct Context {
    pub(crate) download_dir: PathBuf,
    pub(crate) offline_update: bool,
    pub(crate) base_url: String,
}

#[async_trait::async_trait(?Send)]
pub(crate) trait Installer {
    async fn check_requirements(&self, _: &Context) -> Result<()> {
        debug!("running default check_requirements");
        Ok(())
    }

    async fn install(&self, context: &Context) -> Result<()>;
}

#[async_trait::async_trait(?Send)]
impl Installer for Object {
    async fn check_requirements(&self, context: &Context) -> Result<()> {
        for_any_object!(self, o, { o.check_requirements(context).await })
    }

    async fn install(&self, context: &Context) -> Result<()> {
        for_any_object!(self, o, { o.install(context).await })
    }
}

async fn check_if_different<R: AsyncRead + AsyncSeek + Unpin>(
    handle: &mut R,
    rule: &definitions::InstallIfDifferent,
    sha256sum: &str,
) -> Result<bool> {
    match rule {
        definitions::InstallIfDifferent::CheckSum => {
            let mut buffer = Vec::default();
            handle.read_to_end(&mut buffer).await?;
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
            if let Some(ref cur_version) = fbv::version(handle, pattern).await {
                if version == cur_version {
                    return Ok(true);
                }
            }
        }
        definitions::InstallIfDifferent::CustomPattern { version, pattern } => {
            handle.seek(io::SeekFrom::Start(pattern.seek)).await?;
            let mut src = BufReader::with_capacity(pattern.buffer_size as usize, handle);
            if let Some(ref cur_version) =
                fbv::version_with_pattern(&mut src, &pattern.regexp).await
            {
                if version == cur_version {
                    return Ok(true);
                }
            }
        }
    }
    Ok(false)
}

async fn should_skip_install<F, R>(
    rule: &Option<definitions::InstallIfDifferent>,
    sha256sum: &str,
    handler: F,
) -> Result<bool>
where
    F: std::future::Future<Output = Result<R>>,
    R: AsyncRead + AsyncSeek + Unpin,
{
    match rule {
        None => {
            trace!("no install if different rule set, proceeding");
            Ok(false)
        }
        Some(ref rule) => {
            let mut h = handler.await?;
            match check_if_different(&mut h, rule, sha256sum).await {
                Ok(true) => {
                    info!(
                        "installation of {} has been skipped (install if different): {}",
                        sha256sum, rule,
                    );
                    Ok(true)
                }
                Ok(false) => {
                    debug!("installation will proceed (installation if different): {}", rule);
                    Ok(false)
                }
                Err(e) => {
                    error!("install if different check ({}) check failed, error: {}", rule, e);
                    Err(e)
                }
            }
        }
    }
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

    #[tokio::test]
    async fn unmatched_checksum() {
        let f = tempfile::NamedTempFile::new().unwrap();
        let mut h = tokio::fs::File::open(f.path()).await.unwrap();
        assert!(
            !check_if_different(
                &mut h,
                &definitions::InstallIfDifferent::CheckSum,
                "some_sha256sum"
            )
            .await
            .unwrap(),
            "Empty fille should not be validated to the checksum"
        );
    }

    #[tokio::test]
    async fn checksum() {
        let f = tempfile::NamedTempFile::new().unwrap();
        tokio::fs::write(f.path(), b"some_sha256sum").await.unwrap();
        let mut h = tokio::fs::File::open(f.path()).await.unwrap();
        assert!(
            check_if_different(
                &mut h,
                &definitions::InstallIfDifferent::CheckSum,
                "7dc201ce54a835790d78835363a0bce4db704dd23c0c05e399d2a7d1f8fcef19",
            )
            .await
            .unwrap(),
            "Empty fille should not be validated to the checksum"
        );
    }
}
