// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    actor::{self, SharedState},
    Idle, Poll, PrepareDownload, State, StateChangeImpl, StateMachine,
};
use crate::client::{Api, ProbeResponse};
use chrono::{Duration, Utc};
use slog_scope::{debug, error, info};

#[derive(Debug, PartialEq, Clone)]
pub(super) enum ServerAddress {
    Default,
    Custom(String),
}

#[derive(Debug, PartialEq)]
pub(super) struct Probe {
    pub(super) server_address: ServerAddress,
}

create_state_step!(Probe => Idle);
create_state_step!(Probe => Poll);

/// Implements the state change for State<Probe>.
impl StateChangeImpl for State<Probe> {
    fn name(&self) -> &'static str {
        "probe"
    }

    fn handle_trigger_probe(&self) -> actor::probe::Response {
        actor::probe::Response::RequestAccepted(self.name().to_owned())
    }

    fn handle(
        self,
        shared_state: &mut SharedState,
    ) -> Result<(StateMachine, actor::StepTransition), failure::Error> {
        let server_address = match self.0.server_address.clone() {
            ServerAddress::Default => shared_state.settings.network.server_address.clone(),
            ServerAddress::Custom(s) => s,
        };

        let probe = match Api::new(&server_address)
            .probe(&shared_state.runtime_settings, &shared_state.firmware)
        {
            Err(e) => {
                error!("{}", e);
                shared_state.runtime_settings.inc_retries();
                return Ok((
                    StateMachine::Probe(self),
                    actor::StepTransition::Delayed(std::time::Duration::from_secs(1)),
                ));
            }
            Ok(probe) => probe,
        };
        shared_state.runtime_settings.clear_retries();

        match probe {
            ProbeResponse::NoUpdate => {
                debug!("Moving to Idle state as no update is available.");

                // Store timestamp of last polling
                shared_state.runtime_settings.set_last_polling(Utc::now())?;
                Ok((
                    StateMachine::Idle(self.into()),
                    actor::StepTransition::Immediate,
                ))
            }

            ProbeResponse::ExtraPoll(s) => {
                info!("Delaying the probing as requested by the server.");
                shared_state
                    .runtime_settings
                    .set_polling_extra_interval(Duration::seconds(s))?;

                debug!("Moving to Poll state due the extra polling interval.");
                Ok((
                    StateMachine::Poll(self.into()),
                    actor::StepTransition::Immediate,
                ))
            }

            ProbeResponse::Update(u) => {
                // Ensure the package is compatible
                u.compatible_with(&shared_state.firmware)?;
                // Store timestamp of last polling
                shared_state.runtime_settings.set_last_polling(Utc::now())?;

                if Some(u.package_uid()) == shared_state.runtime_settings.applied_package_uid() {
                    info!(
                        "Not applying the update package. Same package has already been installed."
                    );
                    debug!("Moving to Idle state as this update package is already installed.");
                    Ok((
                        StateMachine::Idle(self.into()),
                        actor::StepTransition::Immediate,
                    ))
                } else {
                    debug!("Moving to PrepareDownload state to process the update package.");
                    Ok((
                        StateMachine::PrepareDownload(State(PrepareDownload { update_package: u })),
                        actor::StepTransition::Immediate,
                    ))
                }
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::{
        client::tests::{create_mock_server, FakeServer},
        firmware::{
            tests::{create_fake_metadata, FakeDevice},
            Metadata,
        },
        runtime_settings::RuntimeSettings,
        settings::Settings,
    };
    use std::fs;
    use tempfile::NamedTempFile;

    #[test]
    fn update_not_available() {
        let tmpfile = NamedTempFile::new().unwrap();
        let tmpfile = tmpfile.path();
        fs::remove_file(&tmpfile).unwrap();

        let mock = create_mock_server(FakeServer::NoUpdate);

        let settings = Settings::default();
        let runtime_settings = RuntimeSettings::new()
            .load(tmpfile.to_str().unwrap())
            .unwrap();
        let firmware = Metadata::from_path(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap();
        let mut shared_state = SharedState {
            settings,
            runtime_settings,
            firmware,
        };

        let machine = StateMachine::Probe(State(Probe {
            server_address: ServerAddress::Default,
        }))
        .move_to_next_state(&mut shared_state)
        .unwrap()
        .0;

        mock.assert();

        assert_state!(machine, Idle);
    }

    #[test]
    fn update_available() {
        let tmpfile = NamedTempFile::new().unwrap();
        let tmpfile = tmpfile.path();
        fs::remove_file(&tmpfile).unwrap();

        let mock = create_mock_server(FakeServer::HasUpdate);

        let settings = Settings::default();
        let runtime_settings = RuntimeSettings::new()
            .load(tmpfile.to_str().unwrap())
            .unwrap();
        let firmware = Metadata::from_path(&create_fake_metadata(FakeDevice::HasUpdate)).unwrap();
        let mut shared_state = SharedState {
            settings,
            runtime_settings,
            firmware,
        };

        let machine = StateMachine::Probe(State(Probe {
            server_address: ServerAddress::Default,
        }))
        .move_to_next_state(&mut shared_state)
        .unwrap()
        .0;

        mock.assert();

        assert_state!(machine, PrepareDownload);
    }

    #[test]
    fn invalid_hardware() {
        let tmpfile = NamedTempFile::new().unwrap();
        let tmpfile = tmpfile.path();
        fs::remove_file(&tmpfile).unwrap();

        let mock = create_mock_server(FakeServer::InvalidHardware);

        let settings = Settings::default();
        let runtime_settings = RuntimeSettings::new()
            .load(tmpfile.to_str().unwrap())
            .unwrap();
        let firmware =
            Metadata::from_path(&create_fake_metadata(FakeDevice::InvalidHardware)).unwrap();
        let mut shared_state = SharedState {
            settings,
            runtime_settings,
            firmware,
        };

        let machine = StateMachine::Probe(State(Probe {
            server_address: ServerAddress::Default,
        }))
        .move_to_next_state(&mut shared_state);

        mock.assert();

        assert!(machine.is_err(), "Did not catch an incompatible hardware");
    }

    #[test]
    fn extra_poll_interval() {
        let tmpfile = NamedTempFile::new().unwrap();
        let tmpfile = tmpfile.path();
        fs::remove_file(&tmpfile).unwrap();

        let mock = create_mock_server(FakeServer::ExtraPoll);

        let settings = Settings::default();
        let runtime_settings = RuntimeSettings::new()
            .load(tmpfile.to_str().unwrap())
            .unwrap();
        let firmware = Metadata::from_path(&create_fake_metadata(FakeDevice::ExtraPoll)).unwrap();
        let mut shared_state = SharedState {
            settings,
            runtime_settings,
            firmware,
        };

        let machine = StateMachine::Probe(State(Probe {
            server_address: ServerAddress::Default,
        }))
        .move_to_next_state(&mut shared_state)
        .unwrap()
        .0;

        mock.assert();

        assert_state!(machine, Poll);
    }

    #[test]
    fn skip_same_package_uid() {
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

        let settings = Settings::default();
        let firmware = Metadata::from_path(&create_fake_metadata(FakeDevice::HasUpdate)).unwrap();
        let mut shared_state = SharedState {
            settings,
            runtime_settings,
            firmware,
        };

        let machine = StateMachine::Probe(State(Probe {
            server_address: ServerAddress::Default,
        }))
        .move_to_next_state(&mut shared_state)
        .unwrap()
        .0;

        mock.assert();

        assert_state!(machine, Idle);
    }

    #[test]
    fn error() {
        let tmpfile = NamedTempFile::new().unwrap();
        let tmpfile = tmpfile.path();
        fs::remove_file(&tmpfile).unwrap();

        // The server here waits for the second request which includes the
        // retries to succeed.
        let mock = create_mock_server(FakeServer::ErrorOnce);

        let settings = Settings::default();
        let runtime_settings = RuntimeSettings::new()
            .load(tmpfile.to_str().unwrap())
            .unwrap();
        let firmware = Metadata::from_path(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap();
        let mut shared_state = SharedState {
            settings,
            runtime_settings,
            firmware,
        };

        let machine = StateMachine::Probe(State(Probe {
            server_address: ServerAddress::Default,
        }))
        .move_to_next_state(&mut shared_state)
        .unwrap()
        .0
        .move_to_next_state(&mut shared_state)
        .unwrap()
        .0;

        mock.assert();

        assert_state!(machine, Idle);
    }
}
