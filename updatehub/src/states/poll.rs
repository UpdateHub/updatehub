// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    actor::{self, SharedState},
    Probe, Result, StateChangeImpl, StateMachine,
};
use chrono::Utc;
use slog_scope::{debug, info};

#[derive(Debug, PartialEq)]
pub(super) struct Poll {}

/// Implements the state change for `State<Poll>`.
///
/// This state is used to control when to go to the `State<Probe>`.
#[async_trait::async_trait(?Send)]
impl StateChangeImpl for Poll {
    fn name(&self) -> &'static str {
        "poll"
    }

    fn is_preemptive_state(&self) -> bool {
        true
    }

    async fn handle(
        self,
        shared_state: &mut SharedState,
    ) -> Result<(StateMachine, actor::StepTransition)> {
        crate::logger::start_memory_logging();

        let interval = shared_state.settings.polling.interval;
        let delay = interval
            - Utc::now().signed_duration_since(shared_state.runtime_settings.last_polling());

        if delay > interval || delay.num_seconds() < 0 {
            info!("forcing to Probe state as we are in time");
            return Ok((StateMachine::Probe(Probe {}), actor::StepTransition::Immediate));
        }

        debug!("moving to Probe state after delay.");
        Ok((StateMachine::Probe(Probe {}), actor::StepTransition::Delayed(delay.to_std().unwrap())))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use chrono::{Duration, Utc};

    #[actix_rt::test]
    async fn normal_delay() {
        let setup = crate::tests::TestEnvironment::build().finish();
        let mut shared_state = setup.gen_shared_state();
        shared_state.runtime_settings.polling.last = Utc::now() - Duration::minutes(10);

        let (machine, trans) =
            StateMachine::Poll(Poll {}).move_to_next_state(&mut shared_state).await.unwrap();

        assert_state!(machine, Probe);
        match trans {
            actor::StepTransition::Delayed(d)
                if d <= shared_state.settings.polling.interval.to_std().unwrap() => {}
            _ => panic!("Unexpected StepTransition: {:?}", trans),
        }
    }

    #[actix_rt::test]
    async fn update_in_time() {
        let setup = crate::tests::TestEnvironment::build().finish();
        let mut shared_state = setup.gen_shared_state();

        let (machine, trans) =
            StateMachine::Poll(Poll {}).move_to_next_state(&mut shared_state).await.unwrap();

        assert_state!(machine, Probe);
        match trans {
            actor::StepTransition::Immediate => {}
            _ => panic!("Unexpected StepTransition: {:?}", trans),
        }
    }

    #[actix_rt::test]
    async fn least_probe_in_the_future() {
        let setup = crate::tests::TestEnvironment::build().finish();
        let mut shared_state = setup.gen_shared_state();
        shared_state.runtime_settings.polling.last = Utc::now() + Duration::days(1);

        let (machine, trans) =
            StateMachine::Poll(Poll {}).move_to_next_state(&mut shared_state).await.unwrap();

        assert_state!(machine, Probe);
        match trans {
            actor::StepTransition::Immediate => {}
            _ => panic!("Unexpected StepTransition: {:?}", trans),
        }
    }
}
