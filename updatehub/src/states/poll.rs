// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    machine::{self, Context},
    Probe, Result, State, StateChangeImpl,
};
use chrono::Utc;
use slog_scope::{debug, info};

#[derive(Debug)]
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

    async fn handle(self, context: &mut Context) -> Result<(State, machine::StepTransition)> {
        crate::logger::start_memory_logging();

        let interval = context.settings.polling.interval;
        let delay =
            interval - Utc::now().signed_duration_since(context.runtime_settings.last_polling());

        if delay > interval || delay.num_seconds() < 0 {
            info!("forcing to Probe state as we are in time");
            return Ok((State::Probe(Probe {}), machine::StepTransition::Immediate));
        }

        debug!("moving to Probe state after delay.");
        Ok((State::Probe(Probe {}), machine::StepTransition::Delayed(delay.to_std().unwrap())))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use chrono::{Duration, Utc};

    #[async_std::test]
    async fn normal_delay() {
        let setup = crate::tests::TestEnvironment::build().finish();
        let mut context = setup.gen_context();
        context.runtime_settings.polling.last = Utc::now() - Duration::minutes(10);

        let (machine, trans) = State::Poll(Poll {}).move_to_next_state(&mut context).await.unwrap();

        assert_state!(machine, Probe);
        match trans {
            machine::StepTransition::Delayed(d)
                if d <= context.settings.polling.interval.to_std().unwrap() => {}
            _ => panic!("Unexpected StepTransition: {:?}", trans),
        }
    }

    #[async_std::test]
    async fn update_in_time() {
        let setup = crate::tests::TestEnvironment::build().finish();
        let mut context = setup.gen_context();

        let (machine, trans) = State::Poll(Poll {}).move_to_next_state(&mut context).await.unwrap();

        assert_state!(machine, Probe);
        match trans {
            machine::StepTransition::Immediate => {}
            _ => panic!("Unexpected StepTransition: {:?}", trans),
        }
    }

    #[async_std::test]
    async fn least_probe_in_the_future() {
        let setup = crate::tests::TestEnvironment::build().finish();
        let mut context = setup.gen_context();
        context.runtime_settings.polling.last = Utc::now() + Duration::days(1);

        let (machine, trans) = State::Poll(Poll {}).move_to_next_state(&mut context).await.unwrap();

        assert_state!(machine, Probe);
        match trans {
            machine::StepTransition::Immediate => {}
            _ => panic!("Unexpected StepTransition: {:?}", trans),
        }
    }
}
