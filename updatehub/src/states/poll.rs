// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{Probe, ServerAddress, SharedState, State, StateChangeImpl, StateMachine};
use chrono::{DateTime, Duration, Utc};
use rand::Rng;
use slog_scope::{debug, info};
use std::{
    sync::{Arc, Condvar, Mutex},
    thread,
};

#[derive(Debug, PartialEq)]
pub(super) struct Poll {}

// create_state_step!(Poll => Probe);

/// Implements the state change for `State<Poll>`.
///
/// This state is used to control when to go to the `State<Probe>`.
impl StateChangeImpl for State<Poll> {
    fn name(&self) -> &'static str {
        "poll"
    }

    fn handle(self, shared_state: &mut SharedState) -> Result<StateMachine, failure::Error> {
        let current_time: DateTime<Utc> = Utc::now();

        if shared_state.runtime_settings.is_polling_forced() {
            debug!("Moving to Probe state as soon as possible.");
            return Ok(StateMachine::Probe(State(Probe {
                server_address: ServerAddress::Default,
            })));
        }

        let last_poll = shared_state
            .runtime_settings
            .last_polling()
            .unwrap_or_else(|| {
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
            return Ok(StateMachine::Probe(State(Probe {
                server_address: ServerAddress::Default,
            })));
        }

        let extra_interval = shared_state.runtime_settings.polling_extra_interval();
        if last_poll + extra_interval.unwrap_or_else(|| Duration::seconds(0)) < current_time {
            debug!("Moving to Probe state as the polling's due extra interval.");
            return Ok(StateMachine::Probe(State(Probe {
                server_address: ServerAddress::Default,
            })));
        }

        let probe = Arc::new((Mutex::new(()), Condvar::new()));
        let probe2 = probe.clone();
        let interval = shared_state.settings.polling.interval;
        thread::spawn(move || {
            let (_, ref cvar) = *probe2;
            thread::sleep(interval.to_std().unwrap());
            cvar.notify_one();
        });

        let (ref lock, ref cvar) = *probe;
        let _ = cvar.wait(lock.lock().unwrap());

        debug!("Moving to Probe state.");
        Ok(StateMachine::Probe(State(Probe {
            server_address: ServerAddress::Default,
        })))
    }
}

#[test]
fn extra_poll_in_past() {
    use super::*;
    use crate::firmware::tests::{create_fake_metadata, FakeDevice};

    let mut settings = Settings::default();
    settings.polling.enabled = true;

    let mut runtime_settings = RuntimeSettings::default();
    runtime_settings
        .set_last_polling(Utc::now() - Duration::seconds(10))
        .unwrap();
    runtime_settings
        .set_polling_extra_interval(Duration::seconds(10))
        .unwrap();

    let firmware = Metadata::from_path(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap();
    let mut shared_state = SharedState {
        settings,
        runtime_settings,
        firmware,
    };

    let machine = StateMachine::Poll(State(Poll {})).move_to_next_state(&mut shared_state);

    assert_state!(machine, Probe);
}

#[test]
fn probe_now() {
    use super::*;
    use crate::firmware::tests::{create_fake_metadata, FakeDevice};

    let mut settings = Settings::default();
    settings.polling.enabled = true;

    let mut runtime_settings = RuntimeSettings::default();
    runtime_settings.set_last_polling(Utc::now()).unwrap();
    runtime_settings
        .force_poll()
        .expect("failed to force polling");

    let firmware = Metadata::from_path(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap();
    let mut shared_state = SharedState {
        settings,
        runtime_settings,
        firmware,
    };

    let machine = StateMachine::Poll(State(Poll {})).move_to_next_state(&mut shared_state);

    assert_state!(machine, Probe);
}

#[test]
fn last_poll_in_future() {
    use super::*;
    use crate::firmware::tests::{create_fake_metadata, FakeDevice};

    let mut settings = Settings::default();
    settings.polling.enabled = true;

    let mut runtime_settings = RuntimeSettings::default();
    runtime_settings
        .set_last_polling(Utc::now() + Duration::days(1))
        .unwrap();

    let firmware = Metadata::from_path(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap();
    let mut shared_state = SharedState {
        settings,
        runtime_settings,
        firmware,
    };

    let machine = StateMachine::Poll(State(Poll {})).move_to_next_state(&mut shared_state);

    assert_state!(machine, Probe);
}

#[test]
fn interval_1_second() {
    use super::*;
    use crate::firmware::tests::{create_fake_metadata, FakeDevice};

    let mut settings = Settings::default();
    settings.polling.enabled = true;
    settings.polling.interval = Duration::seconds(1);

    let mut runtime_settings = RuntimeSettings::default();
    runtime_settings.set_last_polling(Utc::now()).unwrap();

    let firmware = Metadata::from_path(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap();
    let mut shared_state = SharedState {
        settings,
        runtime_settings,
        firmware,
    };

    let machine = StateMachine::Poll(State(Poll {})).move_to_next_state(&mut shared_state);

    assert_state!(machine, Probe);
}

#[test]
fn never_polled() {
    use super::*;
    use crate::firmware::tests::{create_fake_metadata, FakeDevice};

    let mut settings = Settings::default();
    settings.polling.enabled = true;
    settings.polling.interval = Duration::seconds(1);

    let runtime_settings = RuntimeSettings::default();
    let firmware = Metadata::from_path(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap();
    let mut shared_state = SharedState {
        settings,
        runtime_settings,
        firmware,
    };

    let machine = StateMachine::Poll(State(Poll {})).move_to_next_state(&mut shared_state);

    assert_state!(machine, Probe);
}
