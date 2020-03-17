// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    actor::{self, SharedState},
    Probe, Result, State, StateChangeImpl, StateMachine,
};
use chrono::Utc;
use slog_scope::{debug, info};

#[derive(Debug, PartialEq)]
pub(super) struct Poll {}

create_state_step!(Poll => Probe);

/// Implements the state change for `State<Poll>`.
///
/// This state is used to control when to go to the `State<Probe>`.
#[async_trait::async_trait]
impl StateChangeImpl for State<Poll> {
    fn name(&self) -> &'static str {
        "poll"
    }

    async fn handle(
        self,
        shared_state: &mut SharedState,
    ) -> Result<(StateMachine, actor::StepTransition)> {
        crate::logger::start_memory_logging();

        if shared_state.runtime_settings.is_polling_forced() {
            debug!("Moving to Probe state as soon as possible.");
            return Ok((StateMachine::Probe(self.into()), actor::StepTransition::Immediate));
        }

        let interval = shared_state.settings.polling.interval;
        let delay = interval
            - Utc::now().signed_duration_since(shared_state.runtime_settings.last_polling());

        if delay > interval || delay.num_seconds() < 0 {
            info!("Forcing to Probe state as we are in time");
            return Ok((StateMachine::Probe(self.into()), actor::StepTransition::Immediate));
        }

        debug!("Moving to Probe state after delay.");
        Ok((
            StateMachine::Probe(self.into()),
            actor::StepTransition::Delayed(delay.to_std().unwrap()),
        ))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::{
        firmware::{
            tests::{create_fake_metadata, FakeDevice},
            Metadata,
        },
        runtime_settings::RuntimeSettings,
        settings::Settings,
    };
    use chrono::{Duration, Utc};

    #[actix_rt::test]
    async fn normal_delay() {
        let settings = Settings::default();
        let runtime_settings = RuntimeSettings::default();
        let firmware = Metadata::from_path(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap();
        let mut shared_state = SharedState { settings, runtime_settings, firmware };
        shared_state.runtime_settings.polling.last = Utc::now() - Duration::minutes(10);

        let (machine, trans) =
            StateMachine::Poll(State(Poll {})).move_to_next_state(&mut shared_state).await.unwrap();

        assert_state!(machine, Probe);
        match trans {
            actor::StepTransition::Delayed(d)
                if d <= shared_state.settings.polling.interval.to_std().unwrap() => {}
            _ => panic!("Unexpected StepTransition: {:?}", trans),
        }
    }

    #[actix_rt::test]
    async fn update_in_time() {
        let settings = Settings::default();
        let runtime_settings = RuntimeSettings::default();
        let firmware = Metadata::from_path(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap();
        let mut shared_state = SharedState { settings, runtime_settings, firmware };

        let (machine, trans) =
            StateMachine::Poll(State(Poll {})).move_to_next_state(&mut shared_state).await.unwrap();

        assert_state!(machine, Probe);
        match trans {
            actor::StepTransition::Immediate => {}
            _ => panic!("Unexpected StepTransition: {:?}", trans),
        }
    }

    #[actix_rt::test]
    async fn forced_probe() {
        let settings = Settings::default();
        let runtime_settings = RuntimeSettings::default();
        let firmware = Metadata::from_path(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap();
        let mut shared_state = SharedState { settings, runtime_settings, firmware };
        shared_state.runtime_settings.force_poll().unwrap();

        let (machine, trans) =
            StateMachine::Poll(State(Poll {})).move_to_next_state(&mut shared_state).await.unwrap();

        assert_state!(machine, Probe);
        match trans {
            actor::StepTransition::Immediate => {}
            _ => panic!("Unexpected StepTransition: {:?}", trans),
        }
    }

    #[actix_rt::test]
    async fn least_probe_in_the_future() {
        let settings = Settings::default();
        let runtime_settings = RuntimeSettings::default();
        let firmware = Metadata::from_path(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap();
        let mut shared_state = SharedState { settings, runtime_settings, firmware };
        shared_state.runtime_settings.polling.last = Utc::now() + Duration::days(1);

        let (machine, trans) =
            StateMachine::Poll(State(Poll {})).move_to_next_state(&mut shared_state).await.unwrap();

        assert_state!(machine, Probe);
        match trans {
            actor::StepTransition::Immediate => {}
            _ => panic!("Unexpected StepTransition: {:?}", trans),
        }
    }
}
