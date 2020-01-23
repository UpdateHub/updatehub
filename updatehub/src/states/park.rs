// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    actor::{self, SharedState},
    State, StateChangeImpl, StateMachine,
};

use slog_scope::debug;

#[derive(Debug, PartialEq)]
pub(super) struct Park {}

/// Implements the state change for `State<Park>`. It stays in
/// `State<Park>` state.
#[async_trait::async_trait]
impl StateChangeImpl for State<Park> {
    fn name(&self) -> &'static str {
        "park"
    }

    fn handle_trigger_probe(&self) -> actor::probe::Response {
        actor::probe::Response::RequestAccepted(self.name().to_owned())
    }

    async fn handle(
        self,
        _: &mut SharedState,
    ) -> Result<(StateMachine, actor::StepTransition), failure::Error> {
        debug!("Staying on Park state.");
        Ok((StateMachine::Park(self), actor::StepTransition::Never))
    }
}
