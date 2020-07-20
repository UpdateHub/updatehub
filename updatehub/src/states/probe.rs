// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    machine::{self, SharedState},
    EntryPoint, Result, State, StateChangeImpl, Validation,
};
use chrono::Utc;
use cloud::api::ProbeResponse;
use slog_scope::{debug, error, info};
use std::time::Duration;

#[derive(Debug, PartialEq)]
pub(super) struct Probe;

/// Implements the state change for State<Probe>.
#[async_trait::async_trait(?Send)]
impl StateChangeImpl for Probe {
    fn name(&self) -> &'static str {
        "probe"
    }

    fn is_preemptive_state(&self) -> bool {
        true
    }

    async fn handle(
        self,
        shared_state: &mut SharedState,
    ) -> Result<(State, machine::StepTransition)> {
        let server_address = shared_state.server_address();

        let probe = match crate::CloudClient::new(&server_address)
            .probe(
                shared_state.runtime_settings.retries() as u64,
                shared_state.firmware.as_cloud_metadata(),
            )
            .await
        {
            Err(cloud::Error::Http(e))
                if e.downcast_ref::<surf::http::url::ParseError>().is_some() =>
            {
                return Err(cloud::Error::Http(e).into());
            }
            Err(e) => {
                error!("Probe failed: {}", e);
                shared_state.runtime_settings.inc_retries();
                return Ok((
                    State::Probe(self),
                    machine::StepTransition::Delayed(Duration::from_secs(1)),
                ));
            }
            Ok(probe) => probe,
        };
        shared_state.runtime_settings.clear_retries();

        match probe {
            ProbeResponse::NoUpdate => {
                debug!("moving to EntryPoint state as no update is available.");

                // Store timestamp of last polling
                shared_state.runtime_settings.set_last_polling(Utc::now())?;
                Ok((State::EntryPoint(EntryPoint {}), machine::StepTransition::Immediate))
            }

            ProbeResponse::ExtraPoll(s) => {
                info!("delaying the probing as requested by the server.");
                Ok((
                    State::Probe(self),
                    machine::StepTransition::Delayed(Duration::from_secs(s as u64)),
                ))
            }

            ProbeResponse::Update(package, sign) => {
                // Store timestamp of last polling
                shared_state.runtime_settings.set_last_polling(Utc::now())?;

                info!("update received.");
                Ok((
                    State::Validation(Validation { package, sign }),
                    machine::StepTransition::Immediate,
                ))
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::cloud_mock;

    #[async_std::test]
    async fn invalid_uri() {
        let setup = crate::tests::TestEnvironment::build().finish();
        let mut shared_state = setup.gen_shared_state();
        cloud_mock::setup_fake_response(cloud_mock::FakeResponse::InvalidUri);

        let res = State::Probe(Probe {}).move_to_next_state(&mut shared_state).await;

        match res {
            Err(crate::states::TransitionError::Client(_)) => {}
            Err(e) => panic!("Unexpected error returned: {:?}", e),
            Ok(s) => panic!("Unexpected ok state reached: {:?}", s),
        }
    }

    #[async_std::test]
    async fn update_not_available() {
        let setup = crate::tests::TestEnvironment::build().finish();
        let mut shared_state = setup.gen_shared_state();
        cloud_mock::setup_fake_response(cloud_mock::FakeResponse::NoUpdate);

        let machine = State::Probe(Probe {}).move_to_next_state(&mut shared_state).await.unwrap().0;

        assert_state!(machine, EntryPoint);
    }

    #[async_std::test]
    async fn update_available() {
        let setup = crate::tests::TestEnvironment::build().finish();
        let mut shared_state = setup.gen_shared_state();
        cloud_mock::setup_fake_response(cloud_mock::FakeResponse::HasUpdate);

        let machine = State::Probe(Probe {}).move_to_next_state(&mut shared_state).await.unwrap().0;

        assert_state!(machine, Validation);
    }

    #[async_std::test]
    async fn extra_poll_interval() {
        let setup = crate::tests::TestEnvironment::build().finish();
        let mut shared_state = setup.gen_shared_state();
        cloud_mock::setup_fake_response(cloud_mock::FakeResponse::ExtraPoll);

        let machine = State::Probe(Probe {}).move_to_next_state(&mut shared_state).await.unwrap().0;

        assert_state!(machine, Probe);
    }
}
