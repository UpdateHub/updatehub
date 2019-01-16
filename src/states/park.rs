// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::states::{State, StateChangeImpl, StateMachine};

use slog::slog_debug;
use slog_scope::debug;

#[derive(Debug, PartialEq)]
pub(super) struct Park {}

/// Implements the state change for `State<Park>`. It stays in
/// `State<Park>` state.
impl StateChangeImpl for State<Park> {
    fn handle(self) -> Result<StateMachine, failure::Error> {
        debug!("Staying on Park state.");
        Ok(StateMachine::Park(self))
    }
}
