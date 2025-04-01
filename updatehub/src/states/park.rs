// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    Result, State, StateChangeImpl,
    machine::{self, Context},
};
use slog_scope::info;

#[derive(Debug)]
pub(super) struct Park {}

/// Implements the state change for `State<Park>`. It stays in
/// `State<Park>` state.
#[async_trait::async_trait(?Send)]
impl StateChangeImpl for Park {
    fn name(&self) -> &'static str {
        "park"
    }

    fn is_preemptive_state(&self) -> bool {
        true
    }

    async fn handle(self, _: &mut Context) -> Result<(State, machine::StepTransition)> {
        info!("parking state machine");
        crate::logger::stop_memory_logging();
        Ok((State::Park(self), machine::StepTransition::Never))
    }
}
