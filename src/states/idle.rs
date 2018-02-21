// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
// 

use states::{State, StateChangeImpl, StateMachine};

use states::poll::Poll;

#[derive(Debug, PartialEq)]
pub struct Idle {}

/// Implements the state change for `State<Idle>`. It has two
/// possibilities:
///
/// If polling is disabled it stays in `State<Idle>`, otherwise, it moves
/// to `State<Poll>` state.
impl StateChangeImpl for State<Idle> {
    // FIXME: when supporting the HTTP API we need allow going to
    // State<Probe>.
    fn to_next_state(self) -> StateMachine {
        if !self.settings.polling.enabled {
            debug!("Polling is disabled, staying on Idle state.");
            return StateMachine::Idle(self);
        }

        debug!("Polling is enabled, moving to Poll state.");
        StateMachine::Poll(self.into())
    }
}

create_state_step!(Idle => Poll);

#[test]
fn polling_disable() {
    use super::*;
    use firmware::tests::{create_fake_metadata, FakeDevice};

    let mut settings = Settings::default();
    settings.polling.enabled = false;

    let runtime_settings = RuntimeSettings::default();

    let firmware = Metadata::new(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap();

    let machine = StateMachine::Idle(State {
        settings: settings,
        runtime_settings: runtime_settings,
        firmware: firmware,
        applied_package_uid: None,
        state: Idle {},
    }).step();

    assert_state!(machine, Idle);
}

#[test]
fn polling_enabled() {
    use super::*;
    use firmware::tests::{create_fake_metadata, FakeDevice};

    let mut settings = Settings::default();
    settings.polling.enabled = true;

    let runtime_settings = RuntimeSettings::default();

    let firmware = Metadata::new(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap();

    let machine = StateMachine::Idle(State {
        settings: settings,
        runtime_settings: runtime_settings,
        firmware: firmware,
        applied_package_uid: None,
        state: Idle {},
    }).step();

    assert_state!(machine, Poll);
}
