// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    actor::{download_abort, probe},
    State, StateChangeImpl, StateMachine,
};

use slog::slog_debug;
use slog_scope::debug;

#[derive(Debug, PartialEq)]
pub(super) struct Park {}

/// Implements the state change for `State<Park>`. It stays in
/// `State<Park>` state.
impl StateChangeImpl for State<Park> {
    fn name(&self) -> &'static str {
        "park"
    }

    fn handle_download_abort(&self) -> download_abort::Response {
        download_abort::Response::InvalidState
    }

    fn handle_trigger_probe(&self) -> probe::Response {
        probe::Response::RequestAccepted(self.name().to_owned())
    }

    fn handle(self) -> Result<StateMachine, failure::Error> {
        debug!("Staying on Park state.");
        Ok(StateMachine::Park(self))
    }
}
