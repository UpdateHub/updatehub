// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    actor::{self, SharedState},
    EntryPoint, Result, State, StateChangeImpl, TransitionError,
};

use crate::firmware;
use slog_scope::{error, info};

#[derive(Debug)]
pub(super) struct Error {
    error: TransitionError,
}

impl PartialEq for Error {
    fn eq(&self, _other: &Self) -> bool {
        // error field intentionally ignored
        true
    }
}

#[async_trait::async_trait(?Send)]
impl StateChangeImpl for Error {
    fn name(&self) -> &'static str {
        "error"
    }

    async fn handle(self, st: &mut SharedState) -> Result<(State, actor::StepTransition)> {
        error!("error state reached: {}", self.error);

        if let Err(err) = firmware::error_callback(&st.settings.firmware.metadata) {
            error!("failed to run error callback script: {}", err);
        }

        info!("returning to machine's entry point");
        Ok((State::EntryPoint(EntryPoint {}), actor::StepTransition::Immediate))
    }
}

impl From<TransitionError> for State {
    fn from(error: TransitionError) -> State {
        State::Error(Error { error })
    }
}
