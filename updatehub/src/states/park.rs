// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    actor::{self, SharedState},
    Result, State, StateChangeImpl, StateMachine,
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

    fn can_run_trigger_probe(&self) -> bool {
        true
    }

    fn can_run_local_install(&self) -> bool {
        true
    }

    fn can_run_remote_install(&self) -> bool {
        true
    }

    async fn handle(self, _: &mut SharedState) -> Result<(StateMachine, actor::StepTransition)> {
        debug!("Staying on Park state.");
        crate::logger::stop_memory_logging();
        Ok((StateMachine::Park(self), actor::StepTransition::Never))
    }
}
