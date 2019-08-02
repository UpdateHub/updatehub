// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::{
    object::{Info, Installer},
    utils,
};
use easy_process;
use failure::format_err;
use pkg_schema::objects;
use slog_scope::info;

impl Installer for objects::Imxkobs {
    fn check_requirements(&self) -> Result<(), failure::Error> {
        info!("'imxkobs' handle checking requirements");
        utils::fs::is_executable_in_path("kobs-ng")?;

        Ok(())
    }

    fn install(&self, download_dir: &std::path::Path) -> Result<(), failure::Error> {
        info!("'imxkobs' handler Install");
        let mut cmd = String::from("kobs-ng init");

        if self.padding_1k {
            cmd += " -x"
        };

        cmd += download_dir
            .join(self.sha256sum())
            .to_str()
            .ok_or_else(|| format_err!("Unable to get source path"))?;

        if self.search_exponent > 0 {
            cmd += &format!(" --search_exponent={}", self.search_exponent)
        }

        if !self.chip_0_device_path.is_empty() {
            cmd += &format!(" --chip_0_device_path={}", self.chip_0_device_path);
        }

        if !self.chip_1_device_path.is_empty() {
            cmd += &format!(" --chip_1_device_path={}", self.chip_1_device_path);
        }

        easy_process::run(&cmd)?;
        Ok(())
    }
}
