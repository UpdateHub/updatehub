// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    machine::{self, Context},
    Park, Poll, Probe, Result, State, StateChangeImpl,
};
use slog_scope::{debug, info};

#[derive(Debug)]
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

    async fn handle(self, context: &mut Context) -> Result<(State, machine::StepTransition)> {
        if context.runtime_settings.is_polling_forced() {
            info!("triggering Probe to finish update.");
            context.runtime_settings.disable_force_poll()?;
            return Ok((State::Probe(Probe {}), machine::StepTransition::Immediate));
        }

        // Cleanup temporary settings from last installation
        context.runtime_settings.reset_transient_settings();

        if !context.settings.polling.enabled {
            debug!("polling is disabled, parking the state machine.");
            return Ok((State::Park(Park {}), machine::StepTransition::Immediate));
        }

        debug!("polling is enabled, moving to Poll state.");
        Ok((State::Poll(Poll {}), machine::StepTransition::Immediate))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[async_std::test]
    async fn polling_disable() {
        let setup = crate::tests::TestEnvironment::build().disable_polling().finish();
        let mut context = setup.gen_context();

        let machine =
            State::EntryPoint(EntryPoint {}).move_to_next_state(&mut context).await.unwrap().0;

        assert_state!(machine, Park);
    }

    #[async_std::test]
    async fn polling_enabled() {
        let setup = crate::tests::TestEnvironment::build().finish();
        let mut context = setup.gen_context();

        let machine =
            State::EntryPoint(EntryPoint {}).move_to_next_state(&mut context).await.unwrap().0;

        assert_state!(machine, Poll);
    }

    #[async_std::test]
    async fn forced_probe() {
        let setup = crate::tests::TestEnvironment::build().finish();
        let mut context = setup.gen_context();
        context.runtime_settings.reset_installation_settings().unwrap();

        let (machine, trans) =
            State::EntryPoint(EntryPoint {}).move_to_next_state(&mut context).await.unwrap();

        assert_state!(machine, Probe);
        match trans {
            machine::StepTransition::Immediate => {}
            _ => panic!("Unexpected StepTransition: {:?}", trans),
        }
    }
}
