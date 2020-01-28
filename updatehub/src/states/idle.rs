// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    actor::{self, SharedState},
    Park, Poll, Result, State, StateChangeImpl, StateMachine,
};
use slog_scope::debug;

#[derive(Debug, PartialEq)]
pub(super) struct Idle {}

/// Implements the state change for `State<Idle>`. It has two
/// possibilities:
///
/// If polling is disabled it stays in `State<Idle>`, otherwise, it moves
/// to `State<Poll>` state.
#[async_trait::async_trait]
impl StateChangeImpl for State<Idle> {
    fn name(&self) -> &'static str {
        "idle"
    }

    fn handle_trigger_probe(&self) -> actor::probe::Response {
        actor::probe::Response::RequestAccepted(self.name().to_owned())
    }

    async fn handle(
        self,
        shared_state: &mut SharedState,
    ) -> Result<(StateMachine, actor::StepTransition)> {
        // Cleanup temporary settings from last installation
        shared_state.runtime_settings.reset_transient_settings();

        if !shared_state.settings.polling.enabled {
            debug!("Polling is disabled, staying on Idle state.");
            return Ok((StateMachine::Park(self.into()), actor::StepTransition::Immediate));
        }

        debug!("Polling is enabled, moving to Poll state.");
        Ok((StateMachine::Poll(self.into()), actor::StepTransition::Immediate))
    }
}

create_state_step!(Idle => Park);
create_state_step!(Idle => Poll);

#[actix_rt::test]
async fn polling_disable() {
    use super::*;
    use crate::firmware::tests::{create_fake_metadata, FakeDevice};

    let mut settings = Settings::default();
    settings.polling.enabled = false;
    let runtime_settings = RuntimeSettings::default();
    let firmware = Metadata::from_path(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap();
    let mut shared_state = SharedState { settings, runtime_settings, firmware };

    let machine =
        StateMachine::Idle(State(Idle {})).move_to_next_state(&mut shared_state).await.unwrap().0;

    assert_state!(machine, Park);
}

#[actix_rt::test]
async fn polling_enabled() {
    use super::*;
    use crate::firmware::tests::{create_fake_metadata, FakeDevice};

    let mut settings = Settings::default();
    settings.polling.enabled = true;
    let runtime_settings = RuntimeSettings::default();
    let firmware = Metadata::from_path(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap();
    let mut shared_state = SharedState { settings, runtime_settings, firmware };

    let machine =
        StateMachine::Idle(State(Idle {})).move_to_next_state(&mut shared_state).await.unwrap().0;

    assert_state!(machine, Poll);
}
