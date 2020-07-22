// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    machine::{self, Context},
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

    async fn handle(self, context: &mut Context) -> Result<(State, machine::StepTransition)> {
        let server_address = context.server_address();

        let probe = match crate::CloudClient::new(&server_address)
            .probe(context.runtime_settings.retries() as u64, context.firmware.as_cloud_metadata())
            .await
        {
            Err(cloud::Error::Http(e))
                if e.downcast_ref::<surf::http::url::ParseError>().is_some() =>
            {
                return Err(cloud::Error::Http(e).into());
            }
            Err(e) => {
                error!("Probe failed: {}", e);
                context.runtime_settings.inc_retries();
                return Ok((
                    State::Probe(self),
                    machine::StepTransition::Delayed(Duration::from_secs(1)),
                ));
            }
            Ok(probe) => probe,
        };
        context.runtime_settings.clear_retries();

        match probe {
            ProbeResponse::NoUpdate => {
                debug!("moving to EntryPoint state as no update is available.");

                // Store timestamp of last polling
                context.runtime_settings.set_last_polling(Utc::now())?;
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
                context.runtime_settings.set_last_polling(Utc::now())?;

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
        let mut context = setup.gen_context();
        cloud_mock::setup_fake_response(cloud_mock::FakeResponse::InvalidUri);

        let res = State::Probe(Probe {}).move_to_next_state(&mut context).await;

        match res {
            Err(crate::states::TransitionError::Client(_)) => {}
            Err(e) => panic!("Unexpected error returned: {:?}", e),
            Ok(s) => panic!("Unexpected ok state reached: {:?}", s),
        }
    }

    #[async_std::test]
    async fn update_not_available() {
        let setup = crate::tests::TestEnvironment::build().finish();
        let mut context = setup.gen_context();
        cloud_mock::setup_fake_response(cloud_mock::FakeResponse::NoUpdate);

        let machine = State::Probe(Probe {}).move_to_next_state(&mut context).await.unwrap().0;

        assert_state!(machine, EntryPoint);
    }

    #[async_std::test]
    async fn update_available() {
        let setup = crate::tests::TestEnvironment::build().finish();
        let mut context = setup.gen_context();
        cloud_mock::setup_fake_response(cloud_mock::FakeResponse::HasUpdate);

        let machine = State::Probe(Probe {}).move_to_next_state(&mut context).await.unwrap().0;

        assert_state!(machine, Validation);
    }

    #[async_std::test]
    async fn extra_poll_interval() {
        let setup = crate::tests::TestEnvironment::build().finish();
        let mut context = setup.gen_context();
        cloud_mock::setup_fake_response(cloud_mock::FakeResponse::ExtraPoll);

        let machine = State::Probe(Probe {}).move_to_next_state(&mut context).await.unwrap().0;

        assert_state!(machine, Probe);
    }
}
