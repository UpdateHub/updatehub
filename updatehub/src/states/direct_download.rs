// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    machine::{self, CommunicationState, Context},
    PrepareLocalInstall, Result, State, StateChangeImpl,
};
use async_lock::Mutex;
use async_std::prelude::FutureExt;
use slog_scope::info;

#[derive(Debug)]
pub(super) struct DirectDownload {
    pub(super) url: String,
}

impl CommunicationState for DirectDownload {}

#[async_trait::async_trait]
impl StateChangeImpl for DirectDownload {
    fn name(&self) -> &'static str {
        "direct_download"
    }

    fn is_handling_download(&self) -> bool {
        true
    }

    async fn handle(self, context: &mut Context) -> Result<(State, machine::StepTransition)> {
        info!("fetching update package directly from url: {:?}", self.url);
        use std::ops::DerefMut;
        let communication_receiver = &context.communication.receiver.clone();
        let context = Mutex::new(context);

        let download_future = async {
            let download_dir = context.lock().await.settings.update.download_dir.clone();
            async_std::fs::create_dir_all(&download_dir).await?;
            let update_file = download_dir.join("fetched_pkg");
            let mut file = async_std::fs::File::create(&update_file).await?;
            cloud::get(&self.url, &mut file).await?;

            Ok(State::PrepareLocalInstall(PrepareLocalInstall { update_file }))
        };

        let message_handle_future = async {
            while let Ok((msg, responder)) = communication_receiver.recv().await {
                if let Some(new_state) = self
                    .handle_communication(msg, responder, context.lock().await.deref_mut())
                    .await
                {
                    return Ok(new_state);
                }
            }

            Err(super::TransitionError::CommunicationFailed)
        };

        Ok((download_future.race(message_handle_future).await?, machine::StepTransition::Immediate))
    }
}
