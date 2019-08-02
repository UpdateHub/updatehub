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

impl Installer for objects::Ubifs {
    fn check_requirements(&self) -> Result<(), failure::Error> {
        info!("'ubifs' handle checking requirements");
        utils::fs::is_executable_in_path("ubiupdatevol")?;
        utils::fs::is_executable_in_path("ubinfo")?;

        if let definitions::TargetType::UBIVolume(_) = self.target.valid()? {
            return Ok(());
        }

        bail!("Unexpected target type, expected some device.")
    }

    fn install(&self, download_dir: &std::path::Path) -> Result<(), failure::Error> {
        info!("'ubifs' handler Install");

        let target = self.target.get_target()?;
        let source = download_dir.join(self.sha256sum());

        easy_process::run(&format!("ubiupdatevol {:?} {:?}", target, source))?;

        Ok(())
    }
}
