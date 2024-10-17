// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{Context, Error, Result};
use crate::{
    object::{Info, Installer},
    utils::{self, log::LogContent},
};
use pkg_schema::objects;
use slog_scope::info;

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

        let source = context.download_dir.join(self.sha256sum());
        let active_install_set = crate::firmware::installation_set::active()
            .log_error_msg("failed to get installation current active set")?;

        // The call to `fw_setenv` here serves to synchronize the U-Boot environment
        // on storage to the new one (which might have been changed). The
        // `libubootenv` tool takes care of avoiding writing to the storage if no
        // changes are required.
        easy_process::run(&format!(
            "fw_setenv -c /etc/fw_env.config --script {}",
            source.to_string_lossy(),
        ))
        .log_error_msg("fw_setenv failed to update")?;
        easy_process::run(&format!(
            "fw_setenv -c /etc/fw_env.config updatehub_active {}",
            active_install_set,
        ))
        .log_error_msg("fw_setenv failed to update updatehub_active")?;

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
        let setup = crate::tests::TestEnvironment::build().add_echo_binary("fw_setenv").finish();

        let uboot_env_obj = fake_uboot_env_obj();
        let download_dir = setup.settings.data.update.download_dir.clone();
        let expected_install_set = Set(InstallationSet::A);
        let source = download_dir.join(&uboot_env_obj.sha256sum);
        let context = Context { download_dir, ..Context::default() };

        uboot_env_obj.install(&context).await.unwrap();

        let output_file = &setup.binaries.data;
        let expected = format!(
            r#"fw_setenv -c /etc/fw_env.config --script {}
fw_setenv -c /etc/fw_env.config updatehub_active {}
"#,
            source.to_string_lossy(),
            expected_install_set,
        );
        assert_eq!(std::fs::read_to_string(output_file).unwrap(), expected);
    }
}
