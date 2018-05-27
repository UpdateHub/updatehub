// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
//

use client::Api;
use failure::Error;
use states::download::Download;
use states::idle::Idle;
use states::poll::Poll;
use states::{State, StateChangeImpl, StateMachine};

#[derive(Debug, PartialEq)]
pub struct Probe {}

create_state_step!(Probe => Idle);
create_state_step!(Probe => Poll);

/// Implements the state change for State<Probe>.
impl StateChangeImpl for State<Probe> {
    fn to_next_state(mut self) -> Result<StateMachine, Error> {
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
            let _ = self.runtime_settings.save().map_err(|e| error!("{:?}", e));
        } else {
            debug!("Skipping runtime settings save, read-only mode enabled.");
        }

        match r {
            ProbeResponse::ExtraPoll(_) => {
                debug!("Moving to Poll state due the extra polling interval.");
                Ok(StateMachine::Poll(self.into()))
            }

            ProbeResponse::Update(u) => {
                if !u.compatible_with(&self.firmware) {
                    debug!("Moving to Idle state as the update package is not compatible.");
                    // FIXME: Must report error
                    return Ok(StateMachine::Idle(self.into()));
                }

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

            ProbeResponse::NoUpdate => {
                debug!("Moving to Idle state as no update is available.");
                Ok(StateMachine::Idle(self.into()))
            }
        }
    }
}

#[test]
fn update_not_available() {
    use super::*;
    use client::tests::{create_mock_server, FakeServer};
    use firmware::tests::{create_fake_metadata, FakeDevice};

    let mock = create_mock_server(FakeServer::NoUpdate);

    let machine = StateMachine::Probe(State {
        settings: Settings::default(),
        runtime_settings: RuntimeSettings::default(),
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

    let mock = create_mock_server(FakeServer::HasUpdate);

    let machine = StateMachine::Probe(State {
        settings: Settings::default(),
        runtime_settings: RuntimeSettings::default(),
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

    let mock = create_mock_server(FakeServer::InvalidHardware);

    let machine = StateMachine::Probe(State {
        settings: Settings::default(),
        runtime_settings: RuntimeSettings::default(),
        firmware: Metadata::new(&create_fake_metadata(FakeDevice::InvalidHardware)).unwrap(),
        applied_package_uid: None,
        state: Probe {},
    }).move_to_next_state();

    mock.assert();

    assert_state!(machine, Idle);
}

#[test]
fn extra_poll_interval() {
    use super::*;
    use client::tests::{create_mock_server, FakeServer};
    use firmware::tests::{create_fake_metadata, FakeDevice};

    let mock = create_mock_server(FakeServer::ExtraPoll);

    let machine = StateMachine::Probe(State {
        settings: Settings::default(),
        runtime_settings: RuntimeSettings::default(),
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

    let mock = create_mock_server(FakeServer::HasUpdate).expect(2);

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
        runtime_settings: RuntimeSettings::default(),
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

    // The server here waits for the second request which includes the
    // retries to succeed.
    let mock = create_mock_server(FakeServer::ErrorOnce);

    let machine = StateMachine::Probe(State {
        settings: Settings::default(),
        runtime_settings: RuntimeSettings::default(),
        firmware: Metadata::new(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap(),
        applied_package_uid: None,
        state: Probe {},
    }).move_to_next_state();

    mock.assert();

    assert_state!(machine, Idle);
}
