// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::states::{Park, Poll, State, StateChangeImpl, StateMachine};

use log::debug;

#[derive(Debug, PartialEq)]
pub(super) struct Idle {}

/// Implements the state change for `State<Idle>`. It has two
/// possibilities:
///
/// If polling is disabled it stays in `State<Idle>`, otherwise, it moves
/// to `State<Poll>` state.
impl StateChangeImpl for State<Idle> {
    // FIXME: when supporting the HTTP API we need allow going to
    // State<Probe>.
    fn handle(self) -> Result<StateMachine, failure::Error> {
        if !self.settings.polling.enabled {
            debug!("Polling is disabled, staying on Idle state.");
            return Ok(StateMachine::Park(self.into()));
        }

        debug!("Polling is enabled, moving to Poll state.");
        Ok(StateMachine::Poll(self.into()))
    }
}

create_state_step!(Idle => Park);
create_state_step!(Idle => Poll);

#[test]
fn polling_disable() {
    use super::*;
    use crate::firmware::tests::{create_fake_metadata, FakeDevice};

    let mut settings = Settings::default();
    settings.polling.enabled = false;

    let machine = StateMachine::Idle(State {
        settings,
        runtime_settings: RuntimeSettings::default(),
        firmware: Metadata::from_path(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap(),
        state: Idle {},
    })
    .move_to_next_state();

    assert_state!(machine, Park);
}

#[test]
fn polling_enabled() {
    use super::*;
    use crate::firmware::tests::{create_fake_metadata, FakeDevice};

    let mut settings = Settings::default();
    settings.polling.enabled = true;

    let machine = StateMachine::Idle(State {
        settings,
        runtime_settings: RuntimeSettings::default(),
        firmware: Metadata::from_path(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap(),
        state: Idle {},
    })
    .move_to_next_state();

    assert_state!(machine, Poll);
}
