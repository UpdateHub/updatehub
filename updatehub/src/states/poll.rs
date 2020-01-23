// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    actor::{self, SharedState},
    Probe, State, StateChangeImpl, StateMachine,
};
use chrono::{DateTime, Duration, Utc};
use rand::Rng;
use slog_scope::{debug, info};

#[derive(Debug, PartialEq)]
pub(super) struct Poll {}

create_state_step!(Poll => Probe);

/// Implements the state change for `State<Poll>`.
///
/// This state is used to control when to go to the `State<Probe>`.
#[async_trait::async_trait]
impl StateChangeImpl for State<Poll> {
    fn name(&self) -> &'static str {
        "poll"
    }

    async fn handle(
        self,
        shared_state: &mut SharedState,
    ) -> Result<(StateMachine, actor::StepTransition), failure::Error> {
        let current_time: DateTime<Utc> = Utc::now();

        if shared_state.runtime_settings.is_polling_forced() {
            debug!("Moving to Probe state as soon as possible.");
            return Ok((StateMachine::Probe(self.into()), actor::StepTransition::Immediate));
        }

        let last_poll = shared_state.runtime_settings.last_polling().unwrap_or_else(|| {
            // When no polling has been done before, we choose an
            // offset between current time and the intended polling
            // interval and use it as last_poll
            let mut rnd = rand::thread_rng();
            let interval = shared_state.settings.polling.interval.num_seconds();
            let offset = Duration::seconds(rnd.gen_range(0, interval));

            current_time + offset
        });

        if last_poll > current_time {
            info!("Forcing to Probe state as last polling seems to happened in future.");
            return Ok((StateMachine::Probe(self.into()), actor::StepTransition::Immediate));
        }

        let extra_interval = shared_state.runtime_settings.polling_extra_interval();
        if last_poll + extra_interval.unwrap_or_else(|| Duration::seconds(0)) > current_time {
            debug!("Moving to Probe state as the polling's due extra interval.");
            return Ok((StateMachine::Probe(self.into()), actor::StepTransition::Immediate));
        }

        debug!("Moving to Probe state after delay.");
        Ok((
            StateMachine::Probe(self.into()),
            actor::StepTransition::Delayed(
                shared_state.settings.polling.interval.to_std().unwrap(),
            ),
        ))
    }
}

#[actix_rt::test]
async fn extra_poll_in_past() {
    use super::*;
    use crate::firmware::tests::{create_fake_metadata, FakeDevice};

    let mut settings = Settings::default();
    settings.polling.enabled = true;

    let mut runtime_settings = RuntimeSettings::default();
    runtime_settings.set_last_polling(Utc::now() - Duration::seconds(10)).unwrap();
    runtime_settings.set_polling_extra_interval(Duration::seconds(20)).unwrap();

    let firmware = Metadata::from_path(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap();
    let mut shared_state = SharedState { settings, runtime_settings, firmware };

    let machine =
        StateMachine::Poll(State(Poll {})).move_to_next_state(&mut shared_state).await.unwrap().0;

    assert_state!(machine, Probe);
}

#[actix_rt::test]
async fn probe_now() {
    use super::*;
    use crate::firmware::tests::{create_fake_metadata, FakeDevice};

    let mut settings = Settings::default();
    settings.polling.enabled = true;

    let mut runtime_settings = RuntimeSettings::default();
    runtime_settings.set_last_polling(Utc::now()).unwrap();
    runtime_settings.force_poll().expect("failed to force polling");

    let firmware = Metadata::from_path(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap();
    let mut shared_state = SharedState { settings, runtime_settings, firmware };

    let machine =
        StateMachine::Poll(State(Poll {})).move_to_next_state(&mut shared_state).await.unwrap().0;

    assert_state!(machine, Probe);
}

#[actix_rt::test]
async fn last_poll_in_future() {
    use super::*;
    use crate::firmware::tests::{create_fake_metadata, FakeDevice};

    let mut settings = Settings::default();
    settings.polling.enabled = true;

    let mut runtime_settings = RuntimeSettings::default();
    runtime_settings.set_last_polling(Utc::now() + Duration::days(1)).unwrap();

    let firmware = Metadata::from_path(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap();
    let mut shared_state = SharedState { settings, runtime_settings, firmware };

    let machine =
        StateMachine::Poll(State(Poll {})).move_to_next_state(&mut shared_state).await.unwrap().0;

    assert_state!(machine, Probe);
}

#[actix_rt::test]
async fn interval_1_second() {
    use super::*;
    use crate::firmware::tests::{create_fake_metadata, FakeDevice};

    let mut settings = Settings::default();
    settings.polling.enabled = true;
    settings.polling.interval = Duration::seconds(1);

    let mut runtime_settings = RuntimeSettings::default();
    runtime_settings.set_last_polling(Utc::now()).unwrap();

    let firmware = Metadata::from_path(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap();
    let mut shared_state = SharedState { settings, runtime_settings, firmware };

    let machine =
        StateMachine::Poll(State(Poll {})).move_to_next_state(&mut shared_state).await.unwrap().0;

    assert_state!(machine, Probe);
}

#[actix_rt::test]
async fn never_polled() {
    use super::*;
    use crate::firmware::tests::{create_fake_metadata, FakeDevice};

    let mut settings = Settings::default();
    settings.polling.enabled = true;
    settings.polling.interval = Duration::seconds(1);

    let runtime_settings = RuntimeSettings::default();
    let firmware = Metadata::from_path(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap();
    let mut shared_state = SharedState { settings, runtime_settings, firmware };

    let machine =
        StateMachine::Poll(State(Poll {})).move_to_next_state(&mut shared_state).await.unwrap().0;

    assert_state!(machine, Probe);
}
