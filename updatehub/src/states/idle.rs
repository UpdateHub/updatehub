// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{actor::probe, Park, Poll, SharedState, State, StateChangeImpl, StateMachine};
use slog_scope::debug;

#[derive(Debug, PartialEq)]
pub(super) struct Idle {}

/// Implements the state change for `State<Idle>`. It has two
/// possibilities:
///
/// If polling is disabled it stays in `State<Idle>`, otherwise, it moves
/// to `State<Poll>` state.
impl StateChangeImpl for State<Idle> {
    fn name(&self) -> &'static str {
        "idle"
    }

    fn handle_trigger_probe(&self) -> probe::Response {
        probe::Response::RequestAccepted(self.name().to_owned())
    }

    fn handle(self, shared_state: &mut SharedState) -> Result<StateMachine, failure::Error> {
        if !shared_state.settings.polling.enabled {
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
    let runtime_settings = RuntimeSettings::default();
    let firmware = Metadata::from_path(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap();
    let mut shared_state = SharedState {
        settings,
        runtime_settings,
        firmware,
    };

    let machine = StateMachine::Idle(State(Idle {})).move_to_next_state(&mut shared_state);

    assert_state!(machine, Park);
}

#[test]
fn polling_enabled() {
    use super::*;
    use crate::firmware::tests::{create_fake_metadata, FakeDevice};

    let mut settings = Settings::default();
    settings.polling.enabled = true;
    let runtime_settings = RuntimeSettings::default();
    let firmware = Metadata::from_path(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap();
    let mut shared_state = SharedState {
        settings,
        runtime_settings,
        firmware,
    };

    let machine = StateMachine::Idle(State(Idle {})).move_to_next_state(&mut shared_state);

    assert_state!(machine, Poll);
}
