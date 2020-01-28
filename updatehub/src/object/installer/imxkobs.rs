// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{Error, Result};
use crate::{
    object::{Info, Installer},
    utils,
};
use easy_process;

use pkg_schema::objects;
use slog_scope::info;

impl Installer for objects::Imxkobs {
    fn check_requirements(&self) -> Result<()> {
        info!("'imxkobs' handle checking requirements");
        utils::fs::is_executable_in_path("kobs-ng")?;

        Ok(())
    }

    fn install(&self, download_dir: &std::path::Path) -> Result<()> {
        info!("'imxkobs' handler Install");
        let mut cmd = String::from("kobs-ng init ");

        if self.padding_1k {
            cmd += "-x "
        };

        cmd += download_dir.join(self.sha256sum()).to_str().ok_or_else(|| Error::InvalidPath)?;

        if self.search_exponent > 0 {
            cmd += &format!(" --search_exponent={}", self.search_exponent)
        }

        if let Some(chip_0) = &self.chip_0_device_path {
            cmd += &format!(" --chip_0_device_path={}", chip_0.display());
        }

        if let Some(chip_1) = &self.chip_1_device_path {
            cmd += &format!(" --chip_1_device_path={}", chip_1.display());
        }

        cmd += " -v";

        easy_process::run(&cmd)?;
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::object::installer::tests::create_echo_bins;
    use pretty_assertions::assert_eq;
    use std::{env, path::PathBuf};

    fn fake_imxkobs_obj() -> objects::Imxkobs {
        objects::Imxkobs {
            filename: "imxkobs-filename".to_string(),
            size: 1024,
            sha256sum: "e3b0c44298fc1c149afb".to_string(),

            install_if_different: None,
            padding_1k: true,
            search_exponent: 2,
            chip_0_device_path: Some(PathBuf::from("/dev/sda1")),
            chip_1_device_path: Some(PathBuf::from("/dev/sda2")),
        }
    }

    #[test]
    fn check_requirements_with_missing_binaries() {
        let imxkobs_obj = fake_imxkobs_obj();

        env::set_var("PATH", "");
        assert!(imxkobs_obj.check_requirements().is_err());
    }

    #[test]
    fn install_no_args() {
        let mut imxkobs_obj = fake_imxkobs_obj();
        imxkobs_obj.padding_1k = false;
        imxkobs_obj.search_exponent = 0;
        imxkobs_obj.chip_0_device_path = None;
        imxkobs_obj.chip_1_device_path = None;
        let download_dir = tempfile::tempdir().unwrap();
        let source = download_dir.path().join(&imxkobs_obj.sha256sum);

        let (_handle, calls) = create_echo_bins(&["kobs-ng"]).unwrap();

        imxkobs_obj.check_requirements().unwrap();
        imxkobs_obj.install(download_dir.path()).unwrap();

        let expected = format!("kobs-ng init {} -v\n", source.to_str().unwrap());
        assert_eq!(std::fs::read_to_string(calls).unwrap(), expected);
    }

    #[test]
    fn install_padding_1k() {
        let mut imxkobs_obj = fake_imxkobs_obj();
        imxkobs_obj.search_exponent = 0;
        imxkobs_obj.chip_0_device_path = None;
        imxkobs_obj.chip_1_device_path = None;
        let download_dir = tempfile::tempdir().unwrap();
        let source = download_dir.path().join(&imxkobs_obj.sha256sum);

        let (_handle, calls) = create_echo_bins(&["kobs-ng"]).unwrap();

        imxkobs_obj.check_requirements().unwrap();
        imxkobs_obj.install(download_dir.path()).unwrap();

        let expected = format!("kobs-ng init -x {} -v\n", source.to_str().unwrap());
        assert_eq!(std::fs::read_to_string(calls).unwrap(), expected);
    }

    #[test]
    fn install_search_exponent() {
        let mut imxkobs_obj = fake_imxkobs_obj();
        imxkobs_obj.padding_1k = false;
        imxkobs_obj.chip_0_device_path = None;
        imxkobs_obj.chip_1_device_path = None;
        let download_dir = tempfile::tempdir().unwrap();
        let source = download_dir.path().join(&imxkobs_obj.sha256sum);

        let (_handle, calls) = create_echo_bins(&["kobs-ng"]).unwrap();

        imxkobs_obj.check_requirements().unwrap();
        imxkobs_obj.install(download_dir.path()).unwrap();

        let expected = format!(
            "kobs-ng init {} --search_exponent={} -v\n",
            source.to_str().unwrap(),
            imxkobs_obj.search_exponent
        );
        assert_eq!(std::fs::read_to_string(calls).unwrap(), expected);
    }

    #[test]
    fn install_chip_0() {
        let mut imxkobs_obj = fake_imxkobs_obj();
        imxkobs_obj.padding_1k = false;
        imxkobs_obj.search_exponent = 0;
        imxkobs_obj.chip_1_device_path = None;
        let download_dir = tempfile::tempdir().unwrap();
        let source = download_dir.path().join(&imxkobs_obj.sha256sum);

        let (_handle, calls) = create_echo_bins(&["kobs-ng"]).unwrap();

        imxkobs_obj.check_requirements().unwrap();
        imxkobs_obj.install(download_dir.path()).unwrap();

        let expected = format!(
            "kobs-ng init {} --chip_0_device_path={} -v\n",
            source.to_str().unwrap(),
            imxkobs_obj.chip_0_device_path.unwrap().display()
        );
        assert_eq!(std::fs::read_to_string(calls).unwrap(), expected);
    }

    #[test]
    fn install_chip_1() {
        let mut imxkobs_obj = fake_imxkobs_obj();
        imxkobs_obj.padding_1k = false;
        imxkobs_obj.search_exponent = 0;
        imxkobs_obj.chip_0_device_path = None;
        let download_dir = tempfile::tempdir().unwrap();
        let source = download_dir.path().join(&imxkobs_obj.sha256sum);

        let (_handle, calls) = create_echo_bins(&["kobs-ng"]).unwrap();

        imxkobs_obj.check_requirements().unwrap();
        imxkobs_obj.install(download_dir.path()).unwrap();

        let expected = format!(
            "kobs-ng init {} --chip_1_device_path={} -v\n",
            source.to_str().unwrap(),
            imxkobs_obj.chip_1_device_path.unwrap().display()
        );
        assert_eq!(std::fs::read_to_string(calls).unwrap(), expected);
    }

    #[test]
    fn install_all_fields() {
        let imxkobs_obj = fake_imxkobs_obj();
        let download_dir = tempfile::tempdir().unwrap();
        let source = download_dir.path().join(&imxkobs_obj.sha256sum);

        let (_handle, calls) = create_echo_bins(&["kobs-ng"]).unwrap();

        imxkobs_obj.check_requirements().unwrap();
        imxkobs_obj.install(download_dir.path()).unwrap();

        let expected = format!(
            "kobs-ng init -x {} --search_exponent={} --chip_0_device_path={} --chip_1_device_path={} -v\n",
            source.to_str().unwrap(),
            imxkobs_obj.search_exponent,
            imxkobs_obj.chip_0_device_path.unwrap().display(),
            imxkobs_obj.chip_1_device_path.unwrap().display()
        );
        assert_eq!(std::fs::read_to_string(calls).unwrap(), expected);
    }
}
