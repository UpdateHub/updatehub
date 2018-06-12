// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
//

use client::Api;
use failure::{Error, ResultExt};
use states::{Download, Idle, Poll, State, StateChangeImpl, StateMachine};

#[derive(Debug, PartialEq)]
pub struct Probe {}

create_state_step!(Probe => Idle);
create_state_step!(Probe => Poll);

/// Implements the state change for State<Probe>.
impl StateChangeImpl for State<Probe> {
    fn handle(mut self) -> Result<StateMachine, Error> {
        use chrono::Duration;
        use client::ProbeResponse;
        use std::thread;

        let r = loop {
            let probe = Api::new(&self.settings, &self.runtime_settings, &self.firmware).probe();
            if let Err(e) = probe {
                error!("{}", e);
                self.runtime_settings.polling.retries += 1;
                thread::sleep(Duration::seconds(1).to_std().unwrap());
            } else {
                self.runtime_settings.polling.retries = 0;
                break probe?;
            }
        };

        self.runtime_settings.polling.extra_interval = match r {
            ProbeResponse::ExtraPoll(s) => {
                info!("Delaying the probing as requested by the server.");
                Some(Duration::seconds(s))
            }
            _ => None,
        };

        // Save any changes we due the probing
        if !self.settings.storage.read_only {
            debug!("Saving runtime settings.");
            self.runtime_settings
                .save()
                .context("Saving runtime due probe changes")?;
        } else {
            debug!("Skipping runtime settings save, read-only mode enabled.");
        }

        match r {
            ProbeResponse::NoUpdate => {
                debug!("Moving to Idle state as no update is available.");
                Ok(StateMachine::Idle(self.into()))
            }

            ProbeResponse::ExtraPoll(_) => {
                debug!("Moving to Poll state due the extra polling interval.");
                Ok(StateMachine::Poll(self.into()))
            }

            ProbeResponse::Update(u) => {
                // Ensure the package is compatible
                u.compatible_with(&self.firmware)?;

                if Some(u.package_uid()) == self.applied_package_uid {
                    info!(
                        "Not applying the update package. Same package has already been installed."
                    );
                    debug!("Moving to Idle state as this update package is already installed.");
                    Ok(StateMachine::Idle(self.into()))
                } else {
                    debug!("Moving to Download state to process the update package.");
                    Ok(StateMachine::Download(State {
                        settings: self.settings,
                        runtime_settings: self.runtime_settings,
                        firmware: self.firmware,
                        applied_package_uid: None,
                        state: Download { update_package: u },
                    }))
                }
            }
        }
    }
}

#[test]
fn update_not_available() {
    use super::*;
    use client::tests::{create_mock_server, FakeServer};
    use firmware::tests::{create_fake_metadata, FakeDevice};
    use mktemp::Temp;

    let mock = create_mock_server(FakeServer::NoUpdate);
    let tmpfile = Temp::new_file().unwrap().to_path_buf();

    let machine = StateMachine::Probe(State {
        settings: Settings::default(),
        runtime_settings: RuntimeSettings::new()
            .load(tmpfile.to_str().unwrap())
            .unwrap(),
        firmware: Metadata::new(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap(),
        applied_package_uid: None,
        state: Probe {},
    }).move_to_next_state();

    mock.assert();

    assert_state!(machine, Idle);
}

#[test]
fn update_available() {
    use super::*;
    use client::tests::{create_mock_server, FakeServer};
    use firmware::tests::{create_fake_metadata, FakeDevice};
    use mktemp::Temp;

    let mock = create_mock_server(FakeServer::HasUpdate);
    let tmpfile = Temp::new_file().unwrap().to_path_buf();

    let machine = StateMachine::Probe(State {
        settings: Settings::default(),
        runtime_settings: RuntimeSettings::new()
            .load(tmpfile.to_str().unwrap())
            .unwrap(),
        firmware: Metadata::new(&create_fake_metadata(FakeDevice::HasUpdate)).unwrap(),
        applied_package_uid: None,
        state: Probe {},
    }).move_to_next_state();

    mock.assert();

    assert_state!(machine, Download);
}

#[test]
fn invalid_hardware() {
    use super::*;
    use client::tests::{create_mock_server, FakeServer};
    use firmware::tests::{create_fake_metadata, FakeDevice};
    use mktemp::Temp;

    let mock = create_mock_server(FakeServer::InvalidHardware);
    let tmpfile = Temp::new_file().unwrap().to_path_buf();

    let machine = StateMachine::Probe(State {
        settings: Settings::default(),
        runtime_settings: RuntimeSettings::new()
            .load(tmpfile.to_str().unwrap())
            .unwrap(),
        firmware: Metadata::new(&create_fake_metadata(FakeDevice::InvalidHardware)).unwrap(),
        applied_package_uid: None,
        state: Probe {},
    }).move_to_next_state();

    mock.assert();

    assert!(machine.is_err(), "Did not catch an incompatible hardware");
}

#[test]
fn extra_poll_interval() {
    use super::*;
    use client::tests::{create_mock_server, FakeServer};
    use firmware::tests::{create_fake_metadata, FakeDevice};
    use mktemp::Temp;

    let mock = create_mock_server(FakeServer::ExtraPoll);
    let tmpfile = Temp::new_file().unwrap().to_path_buf();

    let machine = StateMachine::Probe(State {
        settings: Settings::default(),
        runtime_settings: RuntimeSettings::new()
            .load(tmpfile.to_str().unwrap())
            .unwrap(),
        firmware: Metadata::new(&create_fake_metadata(FakeDevice::ExtraPoll)).unwrap(),
        applied_package_uid: None,
        state: Probe {},
    }).move_to_next_state();

    mock.assert();

    assert_state!(machine, Poll);
}

#[test]
fn skip_same_package_uid() {
    use super::*;
    use client::tests::{create_mock_server, FakeServer};
    use client::ProbeResponse;
    use firmware::tests::{create_fake_metadata, FakeDevice};
    use mktemp::Temp;

    let mock = create_mock_server(FakeServer::HasUpdate).expect(2);
    let tmpfile = Temp::new_file().unwrap().to_path_buf();

    // We first get the package_uid that will be returned so we can
    // use it for the upcoming test.
    //
    // This has been done so we don't need to manually update it every
    // time we change the package payload.
    let package_uid = {
        let probe = Api::new(
            &Settings::default(),
            &RuntimeSettings::default(),
            &Metadata::new(&create_fake_metadata(FakeDevice::HasUpdate)).unwrap(),
        ).probe()
            .unwrap();

        if let ProbeResponse::Update(u) = probe {
            Some(u.package_uid())
        } else {
            None
        }
    };

    let machine = StateMachine::Probe(State {
        settings: Settings::default(),
        runtime_settings: RuntimeSettings::new()
            .load(tmpfile.to_str().unwrap())
            .unwrap(),
        firmware: Metadata::new(&create_fake_metadata(FakeDevice::HasUpdate)).unwrap(),
        applied_package_uid: package_uid,
        state: Probe {},
    }).move_to_next_state();

    mock.assert();

    assert_state!(machine, Idle);
}

#[test]
fn error() {
    use super::*;
    use client::tests::{create_mock_server, FakeServer};
    use firmware::tests::{create_fake_metadata, FakeDevice};
    use mktemp::Temp;

    // The server here waits for the second request which includes the
    // retries to succeed.
    let mock = create_mock_server(FakeServer::ErrorOnce);
    let tmpfile = Temp::new_file().unwrap().to_path_buf();

    let machine = StateMachine::Probe(State {
        settings: Settings::default(),
        runtime_settings: RuntimeSettings::new()
            .load(tmpfile.to_str().unwrap())
            .unwrap(),
        firmware: Metadata::new(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap(),
        applied_package_uid: None,
        state: Probe {},
    }).move_to_next_state();

    mock.assert();

    assert_state!(machine, Idle);
}
