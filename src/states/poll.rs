// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
// 

use chrono::{DateTime, Duration, Utc};
use rand::{self, Rng};
use std::sync::{Arc, Condvar, Mutex};
use std::thread;

use states::{State, StateChangeImpl, StateMachine};

use states::probe::Probe;

#[derive(Debug, PartialEq)]
pub struct Poll {}

create_state_step!(Poll => Probe);

/// Implements the state change for `State<Poll>`.
///
/// This state is used to control when to go to the `State<Probe>`.
impl StateChangeImpl for State<Poll> {
    fn to_next_state(self) -> StateMachine {
        let current_time: DateTime<Utc> = Utc::now();

        let probe_now = self.runtime_settings.polling.now;
        if probe_now {
            debug!("Moving to Probe state as soon as possible.");
            return StateMachine::Probe(self.into());
        }

        let last_poll = self.runtime_settings.polling.last.unwrap_or_else(|| {
            // When no polling has been done before, we choose an
            // offset between current time and the intended polling
            // interval and use it as last_poll
            let mut rnd = rand::thread_rng();
            let interval = self.settings.polling.interval.num_seconds();
            let offset = Duration::seconds(rnd.gen_range(0, interval));

            current_time + offset
        });

        if last_poll > current_time {
            info!("Forcing to Probe state as last polling seems to happened in future.");
            return StateMachine::Probe(self.into());
        }

        let extra_interval = self.runtime_settings.polling.extra_interval;
        if last_poll + extra_interval.unwrap_or(Duration::seconds(0)) < current_time {
            debug!("Moving to Probe state as the polling's due extra interval.");
            return StateMachine::Probe(self.into());
        }

        let probe = Arc::new((Mutex::new(()), Condvar::new()));
        let probe2 = probe.clone();
        let interval = self.settings.polling.interval;
        thread::spawn(move || {
                          let (_, ref cvar) = *probe2;
                          thread::sleep(interval.to_std().unwrap());
                          cvar.notify_one();
                      });

        let (ref lock, ref cvar) = *probe;
        let _ = cvar.wait(lock.lock().unwrap());

        debug!("Moving to Probe state.");
        StateMachine::Probe(self.into())
    }
}

#[test]
fn extra_poll_in_past() {
    use super::*;
    use firmware::tests::{create_fake_metadata, FakeDevice};

    let mut settings = Settings::default();
    settings.polling.enabled = true;

    let mut runtime_settings = RuntimeSettings::default();
    runtime_settings.polling.last = Some(Utc::now() - Duration::seconds(10));
    runtime_settings.polling.extra_interval = Some(Duration::seconds(10));

    let machine = StateMachine::Poll(State { settings: settings,
                                             runtime_settings: runtime_settings,
                                             firmware: Metadata::new(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap(),
                                             applied_package_uid: None,
                                             state: Poll {}, }).step();

    assert_state!(machine, Probe);
}

#[test]
fn probe_now() {
    use super::*;
    use firmware::tests::{create_fake_metadata, FakeDevice};

    let mut settings = Settings::default();
    settings.polling.enabled = true;

    let mut runtime_settings = RuntimeSettings::default();
    runtime_settings.polling.last = Some(Utc::now());
    runtime_settings.polling.now = true;

    let machine = StateMachine::Poll(State { settings: settings,
                                             runtime_settings: runtime_settings,
                                             firmware: Metadata::new(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap(),
                                             applied_package_uid: None,
                                             state: Poll {}, }).step();

    assert_state!(machine, Probe);
}

#[test]
fn last_poll_in_future() {
    use super::*;
    use firmware::tests::{create_fake_metadata, FakeDevice};

    let mut settings = Settings::default();
    settings.polling.enabled = true;

    let mut runtime_settings = RuntimeSettings::default();
    runtime_settings.polling.last = Some(Utc::now() + Duration::days(1));

    let machine = StateMachine::Poll(State { settings: settings,
                                             runtime_settings: runtime_settings,
                                             firmware: Metadata::new(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap(),
                                             applied_package_uid: None,
                                             state: Poll {}, }).step();

    assert_state!(machine, Probe);
}

#[test]
fn interval_1_second() {
    use super::*;
    use firmware::tests::{create_fake_metadata, FakeDevice};

    let mut settings = Settings::default();
    settings.polling.enabled = true;
    settings.polling.interval = Duration::seconds(1);

    let mut runtime_settings = RuntimeSettings::default();
    runtime_settings.polling.last = Some(Utc::now());

    let machine = StateMachine::Poll(State { settings: settings,
                                             runtime_settings: runtime_settings,
                                             firmware: Metadata::new(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap(),
                                             applied_package_uid: None,
                                             state: Poll {}, }).step();

    assert_state!(machine, Probe);
}

#[test]
fn never_polled() {
    use super::*;
    use firmware::tests::{create_fake_metadata, FakeDevice};

    let mut settings = Settings::default();
    settings.polling.enabled = true;
    settings.polling.interval = Duration::seconds(1);

    let machine = StateMachine::Poll(State { settings: settings,
                                             runtime_settings: RuntimeSettings::default(),
                                             firmware: Metadata::new(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap(),
                                             applied_package_uid: None,
                                             state: Poll {}, }).step();

    assert_state!(machine, Probe);
}
