// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    machine::{self, Context},
    EntryPoint, Result, State, StateChangeImpl, Validation,
};
use chrono::{Duration, Utc};
use cloud::api::ProbeResponse;
use slog_scope::{error, info};

#[derive(Debug)]
pub(super) struct Probe;

/// Implements the state change for State<Probe>.
#[async_trait::async_trait]
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
            .probe(context.runtime_settings.retries(), context.firmware.as_cloud_metadata())
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
                    machine::StepTransition::Delayed(Duration::seconds(1)),
                ));
            }
            Ok(probe) => probe,
        };
        context.runtime_settings.clear_retries();

        match probe {
            ProbeResponse::NoUpdate => {
                info!("no update is current available for this device");

                // Store timestamp of last polling
                context.runtime_settings.set_last_polling(Utc::now())?;
                Ok((State::EntryPoint(EntryPoint {}), machine::StepTransition::Immediate))
            }

            ProbeResponse::ExtraPoll(s) => {
                info!("delaying the probing for {} seconds as requested by the server", s);
                Ok((State::Probe(self), machine::StepTransition::Delayed(Duration::seconds(s))))
            }

            ProbeResponse::Update(package, sign) => {
                // Store timestamp of last polling
                context.runtime_settings.set_last_polling(Utc::now())?;

                // Starting logging a new scope of operation since we are
                // beginning the installation process of a new update package
                crate::logger::start_memory_logging();

                info!("update received: {} ({})", package.version(), package.package_uid());
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
