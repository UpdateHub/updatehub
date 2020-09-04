// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{Error, Result};
use crate::{
    object::{Info, Installer},
    utils,
};
use pkg_schema::objects;
use slog_scope::info;

impl Installer for objects::UbootEnv {
    fn check_requirements(&self) -> Result<()> {
        info!("'uboot-env' handle checking requirements");

        utils::fs::is_executable_in_path("fw_setenv")?;
        if !easy_process::run("fw_setenv --help")?.stderr.contains("--script") {
            return Err(Error::FwSetEnvNoScriptOption);
        }

        Ok(())
    }

    fn install(&self, download_dir: &std::path::Path) -> Result<()> {
        info!("'uboot-env' handler Install {} ({})", self.filename, self.sha256sum);

        let source = download_dir.join(self.sha256sum());
        let active_install_set = crate::firmware::installation_set::active()?;

        // The call to `fw_setenv` here serves to synchronize the U-Boot environment
        // on storage to the new one (which might have been changed). The
        // `libubootenv` tool takes care of avoiding writing to the storage if no
        // changes are required.
        easy_process::run(&format!(
            "fw_setenv -c /etc/fw_env.config --script {}",
            source.to_string_lossy(),
        ))?;
        easy_process::run(&format!(
            "fw_setenv -c /etc/fw_env.config updatehub_active {}",
            active_install_set,
        ))?;

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

    #[test]
    fn check_requirements_with_missing_binary() {
        let uboot_env_obj = fake_uboot_env_obj();

        std::env::set_var("PATH", "");
        assert!(uboot_env_obj.check_requirements().is_err());
    }

    #[test]
    fn install() {
        let setup = crate::tests::TestEnvironment::build().add_echo_binary("fw_setenv").finish();
        let output_file = &setup.binaries.data;

        std::fs::write(
            setup.binaries.stored_path.join("fw_setenv"),
            format!(
                r#"#! /bin/bash
case $1 in
  "--help")
    echo "--script" >&2
    ;;
  *)
    echo fw_setenv $@ >> {}
    ;;
esac
"#,
                output_file.to_string_lossy()
            ),
        )
        .unwrap();

        let uboot_env_obj = fake_uboot_env_obj();
        let download_dir = setup.settings.data.update.download_dir.clone();
        let expected_install_set = Set(InstallationSet::A);
        let source = download_dir.join(&uboot_env_obj.sha256sum);

        uboot_env_obj.check_requirements().unwrap();
        uboot_env_obj.install(&download_dir).unwrap();

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
