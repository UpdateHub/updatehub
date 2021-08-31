// Copyright (C) 2021 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{Context, Error, Result};
use crate::{
    object::Installer,
    utils::{self, definitions::TargetTypeExt, delta, log::LogContent},
};

use pkg_schema::{definitions, objects};
use slog_scope::info;

#[async_trait::async_trait(?Send)]
impl Installer for objects::RawDelta {
    async fn check_requirements(&self, context: &Context) -> Result<()> {
        info!("'raw-delta' handle checking requirements");

        if let definitions::TargetType::Device(dev) =
            self.target.valid().log_error_msg("device failed validation")?
        {
            let seed = get_seed_path(self, context);
            let required_size = delta::get_required_size(&seed, dev)
                .await
                .log_error_msg("failed to fetch delta required install size")?;
            utils::fs::ensure_disk_space(dev, required_size)
                .log_error_msg("not enough disk space")?;
            return Ok(());
        }
        Err(Error::InvalidTargetType(self.target.clone()))
    }

    async fn install(&self, context: &Context) -> Result<()> {
        info!("'raw-delta' handler Install {} ({})", self.filename, self.sha256sum);

        let target = self.target.get_target().log_error_msg("failed to get target device")?;
        // Clone's chunk size is used from archives definition,
        // so we can ignore this parameter here
        let _ = self.chunk_size.0;
        let source = get_seed_path(self, context);

        delta::clone(&source, &target, self.seek)
            .await
            .log_error_msg("failed to clone from source to target")?;

        Ok(())
    }
}

fn get_seed_path(obj: &objects::RawDelta, context: &Context) -> String {
    if context.offline_update {
        format!("{:?}", context.download_dir.join(&obj.sha256sum))
    } else {
        format!("{}/{}", context.base_url, &obj.sha256sum)
    }
}
