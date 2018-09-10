// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
//

use Result;

use states::{State, StateChangeImpl, StateMachine};

#[derive(Debug, PartialEq)]
pub(super) struct Park {}

/// Implements the state change for `State<Park>`. It stays in
/// `State<Park>` state.
impl StateChangeImpl for State<Park> {
    fn handle(self) -> Result<StateMachine> {
        debug!("Staying on Park state.");
        Ok(StateMachine::Park(self))
    }
}
