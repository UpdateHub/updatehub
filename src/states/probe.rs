// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::{
    client::Api,
    states::{Download, Idle, Poll, State, StateChangeImpl, StateMachine},
};

use slog::{slog_debug, slog_error, slog_info};
use slog_scope::{debug, error, info};

#[derive(Debug, PartialEq)]
pub(super) struct Probe {}

create_state_step!(Probe => Idle);
create_state_step!(Probe => Poll);

/// Implements the state change for State<Probe>.
impl StateChangeImpl for State<Probe> {
    fn handle(mut self) -> Result<StateMachine, failure::Error> {
        use crate::client::ProbeResponse;
        use chrono::{Duration, Utc};
        use std::thread;

        let r = loop {
            let probe = Api::new(&self.settings.network.server_address)
                .probe(&self.runtime_settings, &self.firmware);
            if let Err(e) = probe {
                error!("{}", e);
                self.runtime_settings.inc_retries();
                thread::sleep(Duration::seconds(1).to_std().unwrap());
            } else {
                self.runtime_settings.clear_retries();
                break probe?;
            }
        };

        if let ProbeResponse::ExtraPoll(s) = r {
            info!("Delaying the probing as requested by the server.");
            self.runtime_settings
                .set_polling_extra_interval(Duration::seconds(s))?;
        };

        match r {
            ProbeResponse::NoUpdate => {
                debug!("Moving to Idle state as no update is available.");

                // Store timestamp of last polling
                self.runtime_settings.set_last_polling(Utc::now())?;
                Ok(StateMachine::Idle(self.into()))
            }

            ProbeResponse::ExtraPoll(_) => {
                debug!("Moving to Poll state due the extra polling interval.");
                Ok(StateMachine::Poll(self.into()))
            }

            ProbeResponse::Update(u) => {
                // Ensure the package is compatible
                u.compatible_with(&self.firmware)?;

                // Store timestamp of last polling
                self.runtime_settings.set_last_polling(Utc::now())?;

                if Some(u.package_uid()) == self.runtime_settings.applied_package_uid() {
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
    use crate::{
        client::tests::{create_mock_server, FakeServer},
        firmware::tests::{create_fake_metadata, FakeDevice},
    };
    use std::fs;
    use tempfile::NamedTempFile;

    crate::logger::init(0);
    let tmpfile = NamedTempFile::new().unwrap();
    let tmpfile = tmpfile.path();
    fs::remove_file(&tmpfile).unwrap();

    let mock = create_mock_server(FakeServer::NoUpdate);

    let machine = StateMachine::Probe(State {
        settings: Settings::default(),
        runtime_settings: RuntimeSettings::new()
            .load(tmpfile.to_str().unwrap())
            .unwrap(),
        firmware: Metadata::from_path(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap(),
        state: Probe {},
    })
    .move_to_next_state();

    mock.assert();

    assert_state!(machine, Idle);
}

#[test]
fn update_available() {
    use super::*;
    use crate::{
        client::tests::{create_mock_server, FakeServer},
        firmware::tests::{create_fake_metadata, FakeDevice},
    };
    use std::fs;
    use tempfile::NamedTempFile;

    crate::logger::init(0);
    let tmpfile = NamedTempFile::new().unwrap();
    let tmpfile = tmpfile.path();
    fs::remove_file(&tmpfile).unwrap();

    let mock = create_mock_server(FakeServer::HasUpdate);

    let machine = StateMachine::Probe(State {
        settings: Settings::default(),
        runtime_settings: RuntimeSettings::new()
            .load(tmpfile.to_str().unwrap())
            .unwrap(),
        firmware: Metadata::from_path(&create_fake_metadata(FakeDevice::HasUpdate)).unwrap(),
        state: Probe {},
    })
    .move_to_next_state();

    mock.assert();

    assert_state!(machine, Download);
}

#[test]
fn invalid_hardware() {
    use super::*;
    use crate::{
        client::tests::{create_mock_server, FakeServer},
        firmware::tests::{create_fake_metadata, FakeDevice},
    };
    use std::fs;
    use tempfile::NamedTempFile;

    crate::logger::init(0);
    let tmpfile = NamedTempFile::new().unwrap();
    let tmpfile = tmpfile.path();
    fs::remove_file(&tmpfile).unwrap();

    let mock = create_mock_server(FakeServer::InvalidHardware);

    let machine = StateMachine::Probe(State {
        settings: Settings::default(),
        runtime_settings: RuntimeSettings::new()
            .load(tmpfile.to_str().unwrap())
            .unwrap(),
        firmware: Metadata::from_path(&create_fake_metadata(FakeDevice::InvalidHardware)).unwrap(),
        state: Probe {},
    })
    .move_to_next_state();

    mock.assert();

    assert!(machine.is_err(), "Did not catch an incompatible hardware");
}

#[test]
fn extra_poll_interval() {
    use super::*;
    use crate::{
        client::tests::{create_mock_server, FakeServer},
        firmware::tests::{create_fake_metadata, FakeDevice},
    };
    use std::fs;
    use tempfile::NamedTempFile;

    crate::logger::init(0);
    let tmpfile = NamedTempFile::new().unwrap();
    let tmpfile = tmpfile.path();
    fs::remove_file(&tmpfile).unwrap();

    let mock = create_mock_server(FakeServer::ExtraPoll);

    let machine = StateMachine::Probe(State {
        settings: Settings::default(),
        runtime_settings: RuntimeSettings::new()
            .load(tmpfile.to_str().unwrap())
            .unwrap(),
        firmware: Metadata::from_path(&create_fake_metadata(FakeDevice::ExtraPoll)).unwrap(),
        state: Probe {},
    })
    .move_to_next_state();

    mock.assert();

    assert_state!(machine, Poll);
}

#[test]
fn skip_same_package_uid() {
    use super::*;
    use crate::{
        client::{
            tests::{create_mock_server, FakeServer},
            ProbeResponse,
        },
        firmware::tests::{create_fake_metadata, FakeDevice},
    };
    use std::fs;
    use tempfile::NamedTempFile;

    crate::logger::init(0);
    let tmpfile = NamedTempFile::new().unwrap();
    let tmpfile = tmpfile.path();
    fs::remove_file(&tmpfile).unwrap();

    let mock = create_mock_server(FakeServer::HasUpdate).expect(2);

    let mut runtime_settings = RuntimeSettings::new()
        .load(tmpfile.to_str().unwrap())
        .unwrap();

    // We first get the package_uid that will be returned so we can
    // use it for the upcoming test.
    //
    // This has been done so we don't need to manually update it every
    // time we change the package payload.
    let probe = Api::new(&Settings::default().network.server_address)
        .probe(
            &RuntimeSettings::default(),
            &Metadata::from_path(&create_fake_metadata(FakeDevice::HasUpdate)).unwrap(),
        )
        .unwrap();

    if let ProbeResponse::Update(u) = probe {
        runtime_settings
            .set_applied_package_uid(&u.package_uid())
            .unwrap();
    }

    let machine = StateMachine::Probe(State {
        settings: Settings::default(),
        runtime_settings,
        firmware: Metadata::from_path(&create_fake_metadata(FakeDevice::HasUpdate)).unwrap(),
        state: Probe {},
    })
    .move_to_next_state();

    mock.assert();

    assert_state!(machine, Idle);
}

#[test]
fn error() {
    use super::*;
    use crate::{
        client::tests::{create_mock_server, FakeServer},
        firmware::tests::{create_fake_metadata, FakeDevice},
    };
    use std::fs;
    use tempfile::NamedTempFile;

    crate::logger::init(0);
    let tmpfile = NamedTempFile::new().unwrap();
    let tmpfile = tmpfile.path();
    fs::remove_file(&tmpfile).unwrap();

    // The server here waits for the second request which includes the
    // retries to succeed.
    let mock = create_mock_server(FakeServer::ErrorOnce);

    let machine = StateMachine::Probe(State {
        settings: Settings::default(),
        runtime_settings: RuntimeSettings::new()
            .load(tmpfile.to_str().unwrap())
            .unwrap(),
        firmware: Metadata::from_path(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap(),
        state: Probe {},
    })
    .move_to_next_state();

    mock.assert();

    assert_state!(machine, Idle);
}
