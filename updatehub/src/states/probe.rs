// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    CallbackReporter, EntryPoint, Result, State, StateChangeImpl, Validation,
    machine::{self, Context},
};
use crate::utils::log::LogContent;
use chrono::{Duration, Utc};
use cloud::api::ProbeResponse;
use slog_scope::{error, info};

#[derive(Debug)]
pub(super) struct Probe;

/// How long to wait before retrying a probe that failed.
///
/// The delay doubles on every failure so that a device unable to reach the
/// server -- an offline one, most notably -- backs off instead of retrying
/// every second for as long as it stays offline. It never grows past the
/// polling interval, as probing slower than configured would defeat it, nor
/// past an hour, so a device regaining connectivity notices reasonably soon
/// even when configured with a long interval.
fn retry_delay(retries: usize, polling_interval: Duration) -> Duration {
    const MAX_RETRY_DELAY: i64 = 60 * 60;

    let cap = polling_interval.num_seconds().clamp(1, MAX_RETRY_DELAY);
    // `retries` has just been incremented for the failure being handled, so the
    // first retry waits a single second.
    let exponential = 1i64 << retries.saturating_sub(1).min(12);

    Duration::seconds(exponential.min(cap))
}

#[async_trait::async_trait(?Send)]
impl CallbackReporter for Probe {
    async fn handle_on_transition_cancel(&self, context: &mut machine::Context) -> Result<()> {
        // Set the last polling time or we loop forever as polling interval will not be
        // respected.
        context
            .runtime_settings
            .set_last_polling(Utc::now())
            .log_error_msg("unable to update last polling to runtime settings")?;

        Ok(())
    }

    async fn handle_on_error(&self, context: &mut machine::Context) -> Result<()> {
        self.handle_on_transition_cancel(context).await
    }
}

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

        let probe = match crate::CloudClient::new(server_address)
            .probe(context.runtime_settings.retries(), context.firmware.as_cloud_metadata())
            .await
        {
            Err(err @ cloud::Error::UrlParse(_)) => {
                return Err(err.into());
            }
            Err(e) => {
                // Probing is the one thing an idle device does, so its outcome
                // is recorded even though memory logging is otherwise scoped to
                // update activity. Repeated outcomes are counted rather than
                // stored again, so this stays constant in memory however long
                // the device goes without an update to install.
                crate::logger::record_out_of_scope(|| error!("Probe failed: {}", e));

                context.runtime_settings.inc_retries();
                return Ok((
                    State::Probe(self),
                    machine::StepTransition::Delayed(retry_delay(
                        context.runtime_settings.retries(),
                        context.settings.polling.interval,
                    )),
                ));
            }
            Ok(probe) => probe,
        };
        context.runtime_settings.clear_retries();

        match probe {
            ProbeResponse::NoUpdate => {
                crate::logger::record_out_of_scope(|| {
                    info!("no update is current available for this device")
                });

                // Store timestamp of last polling
                context
                    .runtime_settings
                    .set_last_polling(Utc::now())
                    .log_error_msg("unable to update last polling to runtime settings")?;
                Ok((State::EntryPoint(EntryPoint {}), machine::StepTransition::Immediate))
            }

            ProbeResponse::ExtraPoll(s) => {
                crate::logger::record_out_of_scope(|| {
                    info!("delaying the probing for {} seconds as requested by the server", s)
                });
                Ok((State::Probe(self), machine::StepTransition::Delayed(Duration::seconds(s))))
            }

            ProbeResponse::Update(package, sign) => {
                // Store timestamp of last polling
                context
                    .runtime_settings
                    .set_last_polling(Utc::now())
                    .log_error_msg("failed to update last polling to runtime settings")?;

                // Starting logging a new scope of operation since we are
                // beginning the installation process of a new update package
                crate::logger::start_memory_logging();

                info!("update received: {} ({})", package.version(), package.package_uid());
                Ok((
                    State::Validation(Validation { package, sign, require_download: true }),
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

    #[tokio::test]
    async fn invalid_uri() {
        let setup = crate::tests::TestEnvironment::build().finish();
        let mut context = setup.gen_context();
        cloud_mock::setup_fake_response(cloud_mock::FakeResponse::InvalidUri);

        let res = State::Probe(Probe {}).move_to_next_state(&mut context).await;

        match res {
            Err(crate::states::TransitionError::Client(_)) => {}
            Err(e) => panic!("Unexpected error returned: {e:?}"),
            Ok(s) => panic!("Unexpected ok state reached: {s:?}"),
        }
    }

    #[tokio::test]
    async fn update_not_available() {
        let setup = crate::tests::TestEnvironment::build().finish();
        let mut context = setup.gen_context();
        cloud_mock::setup_fake_response(cloud_mock::FakeResponse::NoUpdate);

        let machine = State::Probe(Probe {}).move_to_next_state(&mut context).await.unwrap().0;

        assert_state!(machine, EntryPoint);
    }

    #[tokio::test]
    async fn update_available() {
        let setup = crate::tests::TestEnvironment::build().finish();
        let mut context = setup.gen_context();
        cloud_mock::setup_fake_response(cloud_mock::FakeResponse::HasUpdate);

        let machine = State::Probe(Probe {}).move_to_next_state(&mut context).await.unwrap().0;

        assert_state!(machine, Validation);
    }

    #[test]
    fn retry_delay_backs_off_within_bounds() {
        let interval = Duration::days(1);

        // Doubling, starting from the historical single second.
        assert_eq!(retry_delay(1, interval), Duration::seconds(1));
        assert_eq!(retry_delay(2, interval), Duration::seconds(2));
        assert_eq!(retry_delay(3, interval), Duration::seconds(4));

        // It saturates instead of growing for as long as the device is offline.
        assert_eq!(retry_delay(usize::MAX, interval), Duration::seconds(60 * 60));

        // And never ends up probing slower than configured.
        let interval = Duration::seconds(60);
        assert_eq!(retry_delay(usize::MAX, interval), interval);
    }

    #[tokio::test]
    async fn extra_poll_interval() {
        let setup = crate::tests::TestEnvironment::build().finish();
        let mut context = setup.gen_context();
        cloud_mock::setup_fake_response(cloud_mock::FakeResponse::ExtraPoll);

        let machine = State::Probe(Probe {}).move_to_next_state(&mut context).await.unwrap().0;

        assert_state!(machine, Probe);
    }
}
