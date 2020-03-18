// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    actor::{self, SharedState},
    EntryPoint, Poll, Result, State, StateChangeImpl, StateMachine, Validation,
};
use crate::client::{self, Api, ProbeResponse};
use chrono::Utc;
use slog_scope::{debug, error, info};
use std::time::Duration;

#[derive(Debug, PartialEq)]
pub(super) struct Probe;

create_state_step!(Probe => EntryPoint);
create_state_step!(Probe => Poll);

/// Implements the state change for State<Probe>.
#[async_trait::async_trait(?Send)]
impl StateChangeImpl for State<Probe> {
    fn name(&self) -> &'static str {
        "probe"
    }

    fn can_run_trigger_probe(&self) -> bool {
        true
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
            Err(client::Error::Http(e)) if e.is::<awc::http::uri::InvalidUri>() => {
                return Err(client::Error::Http(e).into());
            }
            Err(e) => {
                error!("Probe failed: {}", e);
                shared_state.runtime_settings.inc_retries();
                return Ok((
                    StateMachine::Probe(self),
                    actor::StepTransition::Delayed(Duration::from_secs(1)),
                ));
            }
            Ok(probe) => probe,
        };
        shared_state.runtime_settings.clear_retries();

        match probe {
            ProbeResponse::NoUpdate => {
                debug!("Moving to EntryPoint state as no update is available.");

                // Store timestamp of last polling
                shared_state.runtime_settings.set_last_polling(Utc::now())?;
                Ok((StateMachine::EntryPoint(self.into()), actor::StepTransition::Immediate))
            }

            ProbeResponse::ExtraPoll(s) => {
                info!("Delaying the probing as requested by the server.");
                Ok((
                    StateMachine::Probe(self),
                    actor::StepTransition::Delayed(Duration::from_secs(s as u64)),
                ))
            }

            ProbeResponse::Update(package, sign) => {
                // Store timestamp of last polling
                shared_state.runtime_settings.set_last_polling(Utc::now())?;

                info!("Update received.");
                Ok((
                    StateMachine::Validation(State(Validation { package, sign })),
                    actor::StepTransition::Immediate,
                ))
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
    use sdk::api::info::runtime_settings::ServerAddress;
    use std::fs;
    use tempfile::NamedTempFile;

    #[actix_rt::test]
    async fn invalid_uri() {
        let settings = Settings::default();
        let mut runtime_settings = RuntimeSettings::default();
        runtime_settings.polling.server_address = ServerAddress::Custom("FOO".to_string());
        let firmware = Metadata::from_path(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap();
        let mut shared_state = SharedState { settings, runtime_settings, firmware };

        let res = StateMachine::Probe(State(Probe {})).move_to_next_state(&mut shared_state).await;

        match res {
            Err(crate::states::TransitionError::Client(_)) => {}
            Err(e) => panic!("Unexpected error returned: {:?}", e),
            Ok(s) => panic!("Unexpected ok state reached: {:?}", s),
        }
    }

    #[actix_rt::test]
    async fn update_not_available() {
        let tmpfile = NamedTempFile::new().unwrap();
        let tmpfile = tmpfile.path();
        fs::remove_file(&tmpfile).unwrap();

        let mock = create_mock_server(FakeServer::NoUpdate);

        let settings = Settings::default();
        let runtime_settings = RuntimeSettings::load(tmpfile).unwrap();
        let firmware = Metadata::from_path(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap();
        let mut shared_state = SharedState { settings, runtime_settings, firmware };

        let machine = StateMachine::Probe(State(Probe {}))
            .move_to_next_state(&mut shared_state)
            .await
            .unwrap()
            .0;

        mock.assert();

        assert_state!(machine, EntryPoint);
    }

    #[actix_rt::test]
    async fn update_available() {
        let tmpfile = NamedTempFile::new().unwrap();
        let tmpfile = tmpfile.path();
        fs::remove_file(&tmpfile).unwrap();

        let mock = create_mock_server(FakeServer::HasUpdate);

        let settings = Settings::default();
        let runtime_settings = RuntimeSettings::load(tmpfile).unwrap();
        let firmware = Metadata::from_path(&create_fake_metadata(FakeDevice::HasUpdate)).unwrap();
        let mut shared_state = SharedState { settings, runtime_settings, firmware };

        let machine = StateMachine::Probe(State(Probe {}))
            .move_to_next_state(&mut shared_state)
            .await
            .unwrap()
            .0;

        mock.assert();

        assert_state!(machine, Validation);
    }

    #[actix_rt::test]
    async fn extra_poll_interval() {
        let tmpfile = NamedTempFile::new().unwrap();
        let tmpfile = tmpfile.path();
        fs::remove_file(&tmpfile).unwrap();

        let mock = create_mock_server(FakeServer::ExtraPoll);

        let settings = Settings::default();
        let runtime_settings = RuntimeSettings::load(tmpfile).unwrap();
        let firmware = Metadata::from_path(&create_fake_metadata(FakeDevice::ExtraPoll)).unwrap();
        let mut shared_state = SharedState { settings, runtime_settings, firmware };

        let machine = StateMachine::Probe(State(Probe {}))
            .move_to_next_state(&mut shared_state)
            .await
            .unwrap()
            .0;

        mock.assert();

        assert_state!(machine, Probe);
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
        let runtime_settings = RuntimeSettings::load(tmpfile).unwrap();
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

        assert_state!(machine, EntryPoint);
    }
}
