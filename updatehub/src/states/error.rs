// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    actor::{self, SharedState},
    Idle, Result, State, StateChangeImpl, StateMachine, TransitionError,
};

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

#[async_trait::async_trait]
impl StateChangeImpl for State<Error> {
    fn name(&self) -> &'static str {
        "error"
    }

    async fn handle(self, _: &mut SharedState) -> Result<(StateMachine, actor::StepTransition)> {
        error!("Error state reached: {:?}", self.0.error);

        info!("Returning to idle state");
        Ok((StateMachine::Idle(State(Idle {})), actor::StepTransition::Immediate))
    }
}

impl From<TransitionError> for StateMachine {
    fn from(error: TransitionError) -> StateMachine {
        StateMachine::Error(State(Error { error }))
    }
}
