// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    machine::{self, SharedState},
    PrepareLocalInstall, Result, State, StateChangeImpl,
};
use slog_scope::info;

#[derive(Debug, PartialEq)]
pub(super) struct DirectDownload {
    pub(super) url: String,
}

#[async_trait::async_trait(?Send)]
impl StateChangeImpl for DirectDownload {
    fn name(&self) -> &'static str {
        "direct_download"
    }

    async fn handle(
        self,
        shared_state: &mut SharedState,
    ) -> Result<(State, machine::StepTransition)> {
        info!("fetching update package directly from url: {:?}", self.url);

        let update_file = shared_state.settings.update.download_dir.join("fetched_pkg");
        let mut file = tokio::fs::File::create(&update_file).await?;
        cloud::get(&self.url, &mut file).await?;

        Ok((
            State::PrepareLocalInstall(PrepareLocalInstall { update_file }),
            machine::StepTransition::Immediate,
        ))
    }
}
