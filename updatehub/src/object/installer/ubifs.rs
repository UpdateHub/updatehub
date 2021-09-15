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

#[async_trait::async_trait(?Send)]
impl Installer for objects::Ubifs {
    async fn check_requirements(&self, _: &Context) -> Result<()> {
        info!("'ubifs' handle checking requirements");

        utils::fs::is_executable_in_path("ubiupdatevol")
            .log_error_msg("ubiupdatevol not on PATH")?;
        utils::fs::is_executable_in_path("ubinfo").log_error_msg("ubinfo not on PATH")?;

        if let definitions::TargetType::UBIVolume(_) =
            self.target.valid().log_error_msg("device failed validation")?
        {
            utils::fs::ensure_disk_space(&self.target.get_target()?, self.required_install_size())
                .log_error_msg("not enough disk space")?;
            return Ok(());
        }

        Err(Error::InvalidTargetType(self.target.clone()))
    }

    async fn install(&self, context: &Context) -> Result<()> {
        info!("'ubifs' handler Install {} ({})", self.filename, self.sha256sum);

        let target = self.target.get_target().log_error_msg("failed to get target device")?;
        let source = context.download_dir.join(self.sha256sum());

        if self.compressed {
            easy_process::run_with_stdin(
                &format!("ubiupdatevol {} -", target.display()),
                |stdin| {
                    let mut file =
                        std::fs::File::open(source).log_error_msg("failed open object")?;
                    compress_tools::uncompress_data(&mut file, stdin)
                        .log_error_msg("failed object to stdin of ubiupdatevol")?;
                    Result::Ok(())
                },
            )
            .log_error_msg("ubiupdatevol failed to run")?;
        } else {
            easy_process::run(&format!("ubiupdatevol {} {}", target.display(), source.display()))
                .log_error_msg("ubiupdatevol failed to run")?;
        }

        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::{
        object::installer::tests::create_echo_bins,
        utils::mtd::tests::{FakeUbi, MtdKind, SERIALIZE},
    };
    use pretty_assertions::assert_eq;
    use std::env;

    fn fake_ubifs_obj(name: &str) -> objects::Ubifs {
        objects::Ubifs {
            filename: "ubifs-filename".to_string(),
            size: 1024,
            sha256sum: "e3b0c44298fc1c149afb".to_string(),
            target: definitions::TargetType::UBIVolume(name.to_string()),

            compressed: false,
            required_uncompressed_size: 2048,
        }
    }

    #[tokio::test]
    async fn check_requirements_with_missing_binaries() {
        let ubifs_obj = fake_ubifs_obj("home");

        env::set_var("PATH", "");
        assert!(ubifs_obj.check_requirements(&Context::default()).await.is_err());

        env::set_var("PATH", "");
        let (_handle, _) = create_echo_bins(&["ubinfo"]).unwrap();
        assert!(ubifs_obj.check_requirements(&Context::default()).await.is_err());

        env::set_var("PATH", "");
        let (_handle, _) = create_echo_bins(&["ubiupdatevol"]).unwrap();
        assert!(ubifs_obj.check_requirements(&Context::default()).await.is_err());
    }

    #[tokio::test]
    #[ignore]
    async fn install() {
        let _mtd_lock = SERIALIZE.lock();
        let _ubi = FakeUbi::new(&["home"], MtdKind::Nor).unwrap();
        let ubifs_obj = fake_ubifs_obj("home");
        let download_dir = tempfile::tempdir().unwrap();
        let target = ubifs_obj.target.get_target().unwrap();
        let source = download_dir.path().join(&ubifs_obj.sha256sum);
        let context =
            Context { download_dir: download_dir.path().to_owned(), ..Context::default() };

        let (_handle, calls) = create_echo_bins(&["ubiupdatevol"]).unwrap();

        ubifs_obj.check_requirements(&context).await.unwrap();
        ubifs_obj.install(&context).await.unwrap();

        let expected = format!("ubiupdatevol {} {}\n", target.display(), source.display());
        assert_eq!(std::fs::read_to_string(calls).unwrap(), expected);
    }
}
