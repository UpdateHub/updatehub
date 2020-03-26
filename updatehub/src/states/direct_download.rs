// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    actor::{self, SharedState},
    PrepareLocalInstall, Result, State, StateChangeImpl, StateMachine,
};
use crate::client;
use slog_scope::info;

#[derive(Debug, PartialEq)]
pub(super) struct DirectDownload {
    pub(super) url: String,
}

#[async_trait::async_trait(?Send)]
impl StateChangeImpl for State<DirectDownload> {
    fn name(&self) -> &'static str {
        "direct_download"
    }

    async fn handle(
        self,
        shared_state: &mut SharedState,
    ) -> Result<(StateMachine, actor::StepTransition)> {
        info!("Fetching update package directly from url: {:?}", self.0.url);

        let update_file = shared_state.settings.update.download_dir.join("fetched_pkg");
        let mut file = tokio::fs::File::create(&update_file).await?;
        client::get(&self.0.url, &mut file).await?;

        Ok((
            StateMachine::PrepareLocalInstall(State(PrepareLocalInstall { update_file })),
            actor::StepTransition::Immediate,
        ))
    }
}
