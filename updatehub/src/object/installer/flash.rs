// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::{
    object::{Info, Installer},
    utils::{self, definitions::TargetTypeExt},
};
use failure::bail;
use pkg_schema::{definitions, objects};
use slog_scope::info;

impl Installer for objects::Flash {
    fn check_requirements(&self) -> Result<(), failure::Error> {
        info!("'flash' handle checking requirements");
        utils::fs::is_executable_in_path("nadwrite")?;
        utils::fs::is_executable_in_path("flashcp")?;
        utils::fs::is_executable_in_path("flash_erase")?;

        match self.target {
            definitions::TargetType::Device(_) | definitions::TargetType::MTDName(_) => {
                self.target.valid().map(|_| ())
            }
            _ => bail!("Unexpected target type, expected some device."),
        }
    }

    fn install(&self, download_dir: &std::path::Path) -> Result<(), failure::Error> {
        info!("'flash' handler Install");

        let target = self.target.get_target()?;
        let source = download_dir.join(self.sha256sum());
        let is_nand = utils::mtd::is_nand(&target)?;

        easy_process::run(&format!("flash_erase {:?} 0 0", target))?;

        if is_nand {
            easy_process::run(&format!("nandwrite -p {:?} {:?}", target, source))?;
        } else {
            easy_process::run(&format!("flashcp {:?} {:?}", source, target))?;
        }

        Ok(())
    }
}
