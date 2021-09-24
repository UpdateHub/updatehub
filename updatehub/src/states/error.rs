// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    machine::{self, Context},
    CallbackReporter, EntryPoint, Result, State, StateChangeImpl, TransitionError,
};

use slog_scope::{error, info};

#[derive(Debug)]
pub(super) struct Error {
    error: TransitionError,
}

impl CallbackReporter for Error {}

#[async_trait::async_trait(?Send)]
impl StateChangeImpl for Error {
    fn name(&self) -> &'static str {
        "error"
    }

    async fn handle(self, _: &mut Context) -> Result<(State, machine::StepTransition)> {
        error!("error state reached: {}", self.error);
        info!("returning to machine's entry point");
        Ok((State::EntryPoint(EntryPoint {}), machine::StepTransition::Immediate))
    }
}

impl From<TransitionError> for State {
    fn from(error: TransitionError) -> State {
        State::Error(Error { error })
    }
}
