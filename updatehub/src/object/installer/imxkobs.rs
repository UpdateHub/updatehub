// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{Context, Error, Result};
use crate::{
    object::{Info, Installer},
    utils::{self, log::LogContent},
};
use pkg_schema::objects;
use slog_scope::info;
use std::{fmt::Write as _, path::PathBuf};

#[async_trait::async_trait(?Send)]
impl Installer for objects::Imxkobs {
    async fn check_requirements(&self, _: &Context) -> Result<()> {
        info!("'imxkobs' handle checking requirements");
        utils::fs::is_executable_in_path("kobs-ng").log_error_msg("kobs-ng not in PATH")?;

        Ok(())
    }

    async fn install(&self, context: &Context) -> Result<()> {
        info!("'imxkobs' handler Install {} ({})", self.filename, self.sha256sum);

        let should_skip_install =
            super::should_skip_install(&self.install_if_different, &self.sha256sum, async {
                let path =
                    self.chip_0_device_path.clone().unwrap_or_else(|| PathBuf::from("/dev/mtd0"));
                let f = path.file_name().ok_or(Error::InvalidPath)?;
                let mut file_name = f.to_os_string();
                file_name.push("ro");
                tokio::fs::File::open(path.with_file_name(file_name)).await.map_err(Error::from)
            })
            .await?;
        if should_skip_install {
            return Ok(());
        }

        let mut cmd = String::from("kobs-ng init ");

        if self.padding_1k {
            cmd += "-x "
        };

        cmd += context
            .download_dir
            .join(self.sha256sum())
            .to_str()
            .ok_or(Error::InvalidPath)
            .log_error_msg("invalid path from download_dir for kobs-ng command")?;

        if self.search_exponent > 0 {
            write!(cmd, " --search_exponent={}", self.search_exponent).unwrap();
        }

        if let Some(chip_0) = &self.chip_0_device_path {
            write!(cmd, " --chip_0_device_path={}", chip_0.display()).unwrap();
        }

        if let Some(chip_1) = &self.chip_1_device_path {
            write!(cmd, " --chip_1_device_path={}", chip_1.display()).unwrap();
        }

        cmd += " -v";

        easy_process::run(&cmd).log_error_msg("kobs-ng command failed")?;
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

    #[tokio::test]
    async fn check_requirements_with_missing_binaries() {
        let imxkobs_obj = fake_imxkobs_obj();

        env::set_var("PATH", "");
        assert!(imxkobs_obj.check_requirements(&Context::default()).await.is_err());
    }

    #[tokio::test]
    async fn install_no_args() {
        let mut imxkobs_obj = fake_imxkobs_obj();
        imxkobs_obj.padding_1k = false;
        imxkobs_obj.search_exponent = 0;
        imxkobs_obj.chip_0_device_path = None;
        imxkobs_obj.chip_1_device_path = None;
        let download_dir = tempfile::tempdir().unwrap();
        let source = download_dir.path().join(&imxkobs_obj.sha256sum);
        let context =
            Context { download_dir: download_dir.path().to_owned(), ..Context::default() };

        let (_handle, calls) = create_echo_bins(&["kobs-ng"]).unwrap();

        imxkobs_obj.check_requirements(&context).await.unwrap();
        imxkobs_obj.install(&context).await.unwrap();

        let expected = format!("kobs-ng init {} -v\n", source.to_str().unwrap());
        assert_eq!(std::fs::read_to_string(calls).unwrap(), expected);
    }

    #[tokio::test]
    async fn install_padding_1k() {
        let mut imxkobs_obj = fake_imxkobs_obj();
        imxkobs_obj.search_exponent = 0;
        imxkobs_obj.chip_0_device_path = None;
        imxkobs_obj.chip_1_device_path = None;
        let download_dir = tempfile::tempdir().unwrap();
        let source = download_dir.path().join(&imxkobs_obj.sha256sum);
        let context =
            Context { download_dir: download_dir.path().to_owned(), ..Context::default() };

        let (_handle, calls) = create_echo_bins(&["kobs-ng"]).unwrap();

        imxkobs_obj.check_requirements(&context).await.unwrap();
        imxkobs_obj.install(&context).await.unwrap();

        let expected = format!("kobs-ng init -x {} -v\n", source.to_str().unwrap());
        assert_eq!(std::fs::read_to_string(calls).unwrap(), expected);
    }

    #[tokio::test]
    async fn install_search_exponent() {
        let mut imxkobs_obj = fake_imxkobs_obj();
        imxkobs_obj.padding_1k = false;
        imxkobs_obj.chip_0_device_path = None;
        imxkobs_obj.chip_1_device_path = None;
        let download_dir = tempfile::tempdir().unwrap();
        let source = download_dir.path().join(&imxkobs_obj.sha256sum);
        let context =
            Context { download_dir: download_dir.path().to_owned(), ..Context::default() };

        let (_handle, calls) = create_echo_bins(&["kobs-ng"]).unwrap();

        imxkobs_obj.check_requirements(&context).await.unwrap();
        imxkobs_obj.install(&context).await.unwrap();

        let expected = format!(
            "kobs-ng init {} --search_exponent={} -v\n",
            source.to_str().unwrap(),
            imxkobs_obj.search_exponent
        );
        assert_eq!(std::fs::read_to_string(calls).unwrap(), expected);
    }

    #[tokio::test]
    async fn install_chip_0() {
        let mut imxkobs_obj = fake_imxkobs_obj();
        imxkobs_obj.padding_1k = false;
        imxkobs_obj.search_exponent = 0;
        imxkobs_obj.chip_1_device_path = None;
        let download_dir = tempfile::tempdir().unwrap();
        let source = download_dir.path().join(&imxkobs_obj.sha256sum);
        let context =
            Context { download_dir: download_dir.path().to_owned(), ..Context::default() };

        let (_handle, calls) = create_echo_bins(&["kobs-ng"]).unwrap();

        imxkobs_obj.check_requirements(&context).await.unwrap();
        imxkobs_obj.install(&context).await.unwrap();

        let expected = format!(
            "kobs-ng init {} --chip_0_device_path={} -v\n",
            source.to_str().unwrap(),
            imxkobs_obj.chip_0_device_path.unwrap().display()
        );
        assert_eq!(std::fs::read_to_string(calls).unwrap(), expected);
    }

    #[tokio::test]
    async fn install_chip_1() {
        let mut imxkobs_obj = fake_imxkobs_obj();
        imxkobs_obj.padding_1k = false;
        imxkobs_obj.search_exponent = 0;
        imxkobs_obj.chip_0_device_path = None;
        let download_dir = tempfile::tempdir().unwrap();
        let source = download_dir.path().join(&imxkobs_obj.sha256sum);
        let context =
            Context { download_dir: download_dir.path().to_owned(), ..Context::default() };

        let (_handle, calls) = create_echo_bins(&["kobs-ng"]).unwrap();

        imxkobs_obj.check_requirements(&context).await.unwrap();
        imxkobs_obj.install(&context).await.unwrap();

        let expected = format!(
            "kobs-ng init {} --chip_1_device_path={} -v\n",
            source.to_str().unwrap(),
            imxkobs_obj.chip_1_device_path.unwrap().display()
        );
        assert_eq!(std::fs::read_to_string(calls).unwrap(), expected);
    }

    #[tokio::test]
    async fn install_all_fields() {
        let imxkobs_obj = fake_imxkobs_obj();
        let download_dir = tempfile::tempdir().unwrap();
        let source = download_dir.path().join(&imxkobs_obj.sha256sum);
        let context =
            Context { download_dir: download_dir.path().to_owned(), ..Context::default() };

        let (_handle, calls) = create_echo_bins(&["kobs-ng"]).unwrap();

        imxkobs_obj.check_requirements(&context).await.unwrap();
        imxkobs_obj.install(&context).await.unwrap();

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
