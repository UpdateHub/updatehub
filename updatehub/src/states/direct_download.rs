// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    actor::{self, SharedState},
    PrepareLocalInstall, Result, State, StateChangeImpl, StateMachine, TransitionError,
};
use slog_scope::{debug, error, info};
use tokio::io::AsyncWriteExt;

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
        let mut response = reqwest::get(&self.0.url).await.map_err(|e| {
            error!("Request error: {}", e);
            TransitionError::InvalidRequest
        })?;
        let length = response.content_length().ok_or_else(|| {
            error!("Invalid response: {:?}", response);
            TransitionError::InvalidRequest
        })?;
        let percent = (length / 100) as usize;
        let mut written: f32 = 0.;
        let mut threshold = 10;
        let mut file = tokio::fs::File::create(&update_file).await?;
        while let Some(chunk) =
            response.chunk().await.map_err(|_| TransitionError::InvalidRequest)?
        {
            file.write_all(&chunk).await?;
            written += chunk.len() as f32 / percent as f32;
            if written as usize >= threshold {
                threshold += 20;
                debug!("{}% of the file has been downloaded", written as usize);
            }
        }
        debug!("100% of the file has been downloaded");

        Ok((
            StateMachine::PrepareLocalInstall(State(PrepareLocalInstall { update_file })),
            actor::StepTransition::Immediate,
        ))
    }
}
