// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    actor::{self, SharedState},
    Idle, Poll, PrepareDownload, Result, State, StateChangeImpl, StateMachine,
};
use crate::client::{Api, ProbeResponse};
use chrono::{Duration, Utc};
use slog_scope::{debug, error, info};

#[derive(Debug, PartialEq)]
pub(super) struct Probe;

create_state_step!(Probe => Idle);
create_state_step!(Probe => Poll);

/// Implements the state change for State<Probe>.
#[async_trait::async_trait]
impl StateChangeImpl for State<Probe> {
    fn name(&self) -> &'static str {
        "probe"
    }

    fn handle_trigger_probe(&self) -> actor::probe::Response {
        actor::probe::Response::RequestAccepted(self.name().to_owned())
    }

    async fn handle(
        self,
        shared_state: &mut SharedState,
    ) -> Result<(StateMachine, actor::StepTransition)> {
        let server_address = shared_state.server_address();

        let probe = match Api::new(&server_address)
            .probe(&shared_state.runtime_settings, &shared_state.firmware)
            .await
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
                Ok((StateMachine::Idle(self.into()), actor::StepTransition::Immediate))
            }

            ProbeResponse::ExtraPoll(s) => {
                info!("Delaying the probing as requested by the server.");
                shared_state.runtime_settings.set_polling_extra_interval(Duration::seconds(s))?;

                debug!("Moving to Poll state due the extra polling interval.");
                Ok((StateMachine::Poll(self.into()), actor::StepTransition::Immediate))
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
                    Ok((StateMachine::Idle(self.into()), actor::StepTransition::Immediate))
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

    #[actix_rt::test]
    async fn update_not_available() {
        let tmpfile = NamedTempFile::new().unwrap();
        let tmpfile = tmpfile.path();
        fs::remove_file(&tmpfile).unwrap();

        let mock = create_mock_server(FakeServer::NoUpdate);

        let settings = Settings::default();
        let runtime_settings = RuntimeSettings::new().load(tmpfile.to_str().unwrap()).unwrap();
        let firmware = Metadata::from_path(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap();
        let mut shared_state = SharedState { settings, runtime_settings, firmware };

        let machine = StateMachine::Probe(State(Probe {}))
            .move_to_next_state(&mut shared_state)
            .await
            .unwrap()
            .0;

        mock.assert();

        assert_state!(machine, Idle);
    }

    #[actix_rt::test]
    async fn update_available() {
        let tmpfile = NamedTempFile::new().unwrap();
        let tmpfile = tmpfile.path();
        fs::remove_file(&tmpfile).unwrap();

        let mock = create_mock_server(FakeServer::HasUpdate);

        let settings = Settings::default();
        let runtime_settings = RuntimeSettings::new().load(tmpfile.to_str().unwrap()).unwrap();
        let firmware = Metadata::from_path(&create_fake_metadata(FakeDevice::HasUpdate)).unwrap();
        let mut shared_state = SharedState { settings, runtime_settings, firmware };

        let machine = StateMachine::Probe(State(Probe {}))
            .move_to_next_state(&mut shared_state)
            .await
            .unwrap()
            .0;

        mock.assert();

        assert_state!(machine, PrepareDownload);
    }

    #[actix_rt::test]
    async fn invalid_hardware() {
        let tmpfile = NamedTempFile::new().unwrap();
        let tmpfile = tmpfile.path();
        fs::remove_file(&tmpfile).unwrap();

        let mock = create_mock_server(FakeServer::InvalidHardware);

        let settings = Settings::default();
        let runtime_settings = RuntimeSettings::new().load(tmpfile.to_str().unwrap()).unwrap();
        let firmware =
            Metadata::from_path(&create_fake_metadata(FakeDevice::InvalidHardware)).unwrap();
        let mut shared_state = SharedState { settings, runtime_settings, firmware };

        let machine =
            StateMachine::Probe(State(Probe {})).move_to_next_state(&mut shared_state).await;

        mock.assert();

        assert!(machine.is_err(), "Did not catch an incompatible hardware");
    }

    #[actix_rt::test]
    async fn extra_poll_interval() {
        let tmpfile = NamedTempFile::new().unwrap();
        let tmpfile = tmpfile.path();
        fs::remove_file(&tmpfile).unwrap();

        let mock = create_mock_server(FakeServer::ExtraPoll);

        let settings = Settings::default();
        let runtime_settings = RuntimeSettings::new().load(tmpfile.to_str().unwrap()).unwrap();
        let firmware = Metadata::from_path(&create_fake_metadata(FakeDevice::ExtraPoll)).unwrap();
        let mut shared_state = SharedState { settings, runtime_settings, firmware };

        let machine = StateMachine::Probe(State(Probe {}))
            .move_to_next_state(&mut shared_state)
            .await
            .unwrap()
            .0;

        mock.assert();

        assert_state!(machine, Poll);
    }

    #[actix_rt::test]
    async fn skip_same_package_uid() {
        let tmpfile = NamedTempFile::new().unwrap();
        let tmpfile = tmpfile.path();
        fs::remove_file(&tmpfile).unwrap();

        let mock = create_mock_server(FakeServer::HasUpdate).expect(2);

        let mut runtime_settings = RuntimeSettings::new().load(tmpfile.to_str().unwrap()).unwrap();

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
            .await
            .unwrap();

        if let ProbeResponse::Update(u) = probe {
            runtime_settings.set_applied_package_uid(&u.package_uid()).unwrap();
        }

        let settings = Settings::default();
        let firmware = Metadata::from_path(&create_fake_metadata(FakeDevice::HasUpdate)).unwrap();
        let mut shared_state = SharedState { settings, runtime_settings, firmware };

        let machine = StateMachine::Probe(State(Probe {}))
            .move_to_next_state(&mut shared_state)
            .await
            .unwrap()
            .0;

        mock.assert();

        assert_state!(machine, Idle);
    }

    #[actix_rt::test]
    async fn error() {
        let tmpfile = NamedTempFile::new().unwrap();
        let tmpfile = tmpfile.path();
        fs::remove_file(&tmpfile).unwrap();

        // The server here waits for the second request which includes the
        // retries to succeed.
        let mock = create_mock_server(FakeServer::ErrorOnce);

        let settings = Settings::default();
        let runtime_settings = RuntimeSettings::new().load(tmpfile.to_str().unwrap()).unwrap();
        let firmware = Metadata::from_path(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap();
        let mut shared_state = SharedState { settings, runtime_settings, firmware };

        let machine = StateMachine::Probe(State(Probe {}))
            .move_to_next_state(&mut shared_state)
            .await
            .unwrap()
            .0
            .move_to_next_state(&mut shared_state)
            .await
            .unwrap()
            .0;

        mock.assert();

        assert_state!(machine, Idle);
    }
}
