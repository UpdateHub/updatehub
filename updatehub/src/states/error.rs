// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    actor::{self, SharedState},
    Idle, State, StateChangeImpl, StateMachine,
};

use derivative::Derivative;
use slog_scope::{error, info};

#[derive(Derivative)]
#[derivative(Debug, PartialEq)]
pub(super) struct Error {
    #[derivative(PartialEq = "ignore")]
    error: failure::Error,
}

#[async_trait::async_trait]
impl StateChangeImpl for State<Error> {
    fn name(&self) -> &'static str {
        "error"
    }

    async fn handle(
        self,
        _: &mut SharedState,
    ) -> Result<(StateMachine, actor::StepTransition), failure::Error> {
        error!("Error state reached: {:?}", self.0.error);

        info!("Returning to idle state");
        Ok((StateMachine::Idle(State(Idle {})), actor::StepTransition::Immediate))
    }
}

impl From<failure::Error> for StateMachine {
    fn from(error: failure::Error) -> StateMachine {
        StateMachine::Error(State(Error { error }))
    }
}
