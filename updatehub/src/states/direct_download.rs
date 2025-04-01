// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    PrepareLocalInstall, Result, State, StateChangeImpl,
    machine::{self, CommunicationState, Context},
};
use crate::utils::log::LogContent;
use async_lock::Mutex;
use slog_scope::info;

#[derive(Debug)]
pub(super) struct DirectDownload {
    pub(super) url: String,
}

impl CommunicationState for DirectDownload {}

#[async_trait::async_trait(?Send)]
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
            tokio::fs::create_dir_all(&download_dir)
                .await
                .log_error_msg("unable to create download dir")?;
            let update_file = download_dir.join("fetched_pkg");
            let mut file = tokio::fs::File::create(&update_file)
                .await
                .log_error_msg("unable to open file for fatching package")?;
            cloud::get(&self.url, &mut file).await.log_error_msg("failed to fetch package")?;

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

        futures_util::pin_mut!(download_future);
        futures_util::pin_mut!(message_handle_future);

        Ok((
            futures_util::future::select(download_future, message_handle_future)
                .await
                .factor_first()
                .0?,
            machine::StepTransition::Immediate,
        ))
    }
}
