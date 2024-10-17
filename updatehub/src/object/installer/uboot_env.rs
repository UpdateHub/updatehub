// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{Context, Error, Result};
use crate::{
    firmware,
    object::{Info, Installer},
    utils::{self, log::LogContent},
};
use pkg_schema::{definitions, objects};
use slog_scope::info;
use tokio::{fs, io};

#[async_trait::async_trait(?Send)]
impl Installer for objects::UbootEnv {
    async fn check_requirements(&self, _: &Context) -> Result<()> {
        info!("'uboot-env' handle checking requirements");

        utils::fs::is_executable_in_path("fw_setenv")
            .log_error_msg("fw_setenv not found in PATH")?;
        if !easy_process::run("fw_setenv --help")
            .log_error_msg("fw_setenv --help failed to run")?
            .stdout
            .contains("--script")
        {
            return Err(Error::FwSetEnvNoScriptOption);
        }

        Ok(())
    }

    async fn install(&self, context: &Context) -> Result<()> {
        info!("'uboot-env' handler Install {} ({})", self.filename, self.sha256sum);

        let chunk_size = definitions::ChunkSize::default().0;

        let source = context.download_dir.join(self.sha256sum());
        let mut input = utils::io::timed_buf_reader(
            chunk_size,
            fs::File::open(source).await.log_error_msg("failed to open source object")?,
        );

        let dest = "/tmp/updatehub.defenv";
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

        io::copy(&mut input, &mut output)
            .await
            .log_error_msg("failed to copy from object to target")?;

        // The call to `firmware::installation_set::set_active` here serves to
        // synchronize the U-Boot environment on storage to the new one (which
        // might have been changed). The `libubootenv` tool takes care of
        let active_install_set = firmware::installation_set::active()
            .log_error_msg("failed to get installation current active set")?;
        // avoiding writing to the storage if no changes are required.
        firmware::installation_set::set_active(active_install_set)
            .log_error_msg("failed to set installation active set")?;

        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::firmware::installation_set::Set;
    use pretty_assertions::assert_eq;
    use sdk::api::info::runtime_settings::InstallationSet;

    fn fake_uboot_env_obj() -> objects::UbootEnv {
        objects::UbootEnv {
            filename: "updatehub.defenv".to_string(),
            size: 1024,
            sha256sum: "e3b0c44298fc1c149afb".to_string(),
        }
    }

    #[tokio::test]
    async fn check_requirements_with_missing_binary() {
        let uboot_env_obj = fake_uboot_env_obj();

        std::env::set_var("PATH", "");
        assert!(uboot_env_obj.check_requirements(&Context::default()).await.is_err());
    }

    #[tokio::test]
    async fn check_requirements_is_ok() {
        let setup = crate::tests::TestEnvironment::build().add_echo_binary("fw_setenv").finish();

        std::fs::write(
            setup.binaries.stored_path.join("fw_setenv"),
            format!(
                r#"#! /bin/sh
case $1 in
  "--help")
    echo "--script"
    ;;
  *)
    echo fw_setenv $@ >> {}
    ;;
esac
"#,
                &setup.binaries.data.to_string_lossy()
            ),
        )
        .unwrap();

        let uboot_env_obj = fake_uboot_env_obj();
        assert!(uboot_env_obj.check_requirements(&Context::default()).await.is_ok());
    }

    #[tokio::test]
    async fn install() {
        let setup =
            crate::tests::TestEnvironment::build().add_echo_binary("updatehub-active-set").finish();

        let uboot_env_obj = fake_uboot_env_obj();
        let download_dir = setup.settings.data.update.download_dir.clone();

        std::fs::write(download_dir.join(&uboot_env_obj.sha256sum), "abc").unwrap();

        let context = Context { download_dir, ..Context::default() };

        uboot_env_obj.install(&context).await.unwrap();

        let expected = format!("updatehub-active-set {}\n", Set(InstallationSet::A));
        assert_eq!(std::fs::read_to_string(&setup.binaries.data).unwrap(), expected);
    }
}
