// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    actor::{self, SharedState},
    Park, Poll, Probe, Result, State, StateChangeImpl,
};
use slog_scope::{debug, info};

#[derive(Debug, PartialEq)]
pub(super) struct EntryPoint {}

/// Implements the state change for `State<EntryPoint>`. It has two
/// possibilities:
///
/// If polling is disabled it stays in `State<EntryPoint>`, otherwise, it moves
/// to `State<Poll>` state.
#[async_trait::async_trait(?Send)]
impl StateChangeImpl for EntryPoint {
    fn name(&self) -> &'static str {
        "entry_point"
    }

    fn is_preemptive_state(&self) -> bool {
        true
    }

    async fn handle(
        self,
        shared_state: &mut SharedState,
    ) -> Result<(State, actor::StepTransition)> {
        if shared_state.runtime_settings.is_polling_forced() {
            info!("triggering Probe to finish update.");
            shared_state.runtime_settings.disable_force_poll()?;
            return Ok((State::Probe(Probe {}), actor::StepTransition::Immediate));
        }

        // Cleanup temporary settings from last installation
        shared_state.runtime_settings.reset_transient_settings();

        if !shared_state.settings.polling.enabled {
            debug!("polling is disabled, parking the state machine.");
            return Ok((State::Park(Park {}), actor::StepTransition::Immediate));
        }

        debug!("polling is enabled, moving to Poll state.");
        Ok((State::Poll(Poll {}), actor::StepTransition::Immediate))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[actix_rt::test]
    async fn polling_disable() {
        let setup = crate::tests::TestEnvironment::build().disable_polling().finish();
        let mut shared_state = setup.gen_shared_state();

        let machine =
            State::EntryPoint(EntryPoint {}).move_to_next_state(&mut shared_state).await.unwrap().0;

        assert_state!(machine, Park);
    }

    #[actix_rt::test]
    async fn polling_enabled() {
        let setup = crate::tests::TestEnvironment::build().finish();
        let mut shared_state = setup.gen_shared_state();

        let machine =
            State::EntryPoint(EntryPoint {}).move_to_next_state(&mut shared_state).await.unwrap().0;

        assert_state!(machine, Poll);
    }

    #[actix_rt::test]
    async fn forced_probe() {
        let setup = crate::tests::TestEnvironment::build().finish();
        let mut shared_state = setup.gen_shared_state();
        shared_state.runtime_settings.reset_installation_settings().unwrap();

        let (machine, trans) =
            State::EntryPoint(EntryPoint {}).move_to_next_state(&mut shared_state).await.unwrap();

        assert_state!(machine, Probe);
        match trans {
            actor::StepTransition::Immediate => {}
            _ => panic!("Unexpected StepTransition: {:?}", trans),
        }
    }
}
