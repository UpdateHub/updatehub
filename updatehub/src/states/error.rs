// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{Idle, SharedState, State, StateChangeImpl, StateMachine};

use derivative::Derivative;
use slog_scope::{error, info};

#[derive(Derivative)]
#[derivative(Debug, PartialEq)]
pub(super) struct Error {
    #[derivative(PartialEq = "ignore")]
    error: failure::Error,
}

impl StateChangeImpl for State<Error> {
    fn name(&self) -> &'static str {
        "error"
    }

    fn handle(self, _: &mut SharedState) -> Result<StateMachine, failure::Error> {
        error!("Error state reached: {:?}", self.0.error);

        info!("Returning to idle state");
        Ok(StateMachine::Idle(State(Idle {})))
    }
}

impl From<failure::Error> for StateMachine {
    fn from(error: failure::Error) -> StateMachine {
        StateMachine::Error(State(Error { error }))
    }
}
