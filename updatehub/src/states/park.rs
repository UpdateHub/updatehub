// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    actor::{self, SharedState},
    Result, State, StateChangeImpl,
};

use slog_scope::debug;

#[derive(Debug, PartialEq)]
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

    async fn handle(self, _: &mut SharedState) -> Result<(State, actor::StepTransition)> {
        debug!("staying on Park state.");
        crate::logger::stop_memory_logging();
        Ok((State::Park(self), actor::StepTransition::Never))
    }
}
