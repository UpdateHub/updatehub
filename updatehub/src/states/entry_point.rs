// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    actor::{self, SharedState},
    Park, Poll, Probe, Result, State, StateChangeImpl, StateMachine,
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
impl StateChangeImpl for State<EntryPoint> {
    fn name(&self) -> &'static str {
        "entry_point"
    }

    fn is_preemptive_state(&self) -> bool {
        true
    }

    async fn handle(
        self,
        shared_state: &mut SharedState,
    ) -> Result<(StateMachine, actor::StepTransition)> {
        if shared_state.runtime_settings.is_polling_forced() {
            info!("triggering Probe to finish update.");
            shared_state.runtime_settings.disable_force_poll()?;
            return Ok((StateMachine::Probe(self.into()), actor::StepTransition::Immediate));
        }

        // Cleanup temporary settings from last installation
        shared_state.runtime_settings.reset_transient_settings();

        if !shared_state.settings.polling.enabled {
            debug!("polling is disabled, parking the state machine.");
            return Ok((StateMachine::Park(self.into()), actor::StepTransition::Immediate));
        }

        debug!("polling is enabled, moving to Poll state.");
        Ok((StateMachine::Poll(self.into()), actor::StepTransition::Immediate))
    }
}

create_state_step!(EntryPoint => Park);
create_state_step!(EntryPoint => Poll);
create_state_step!(EntryPoint => Probe);

#[cfg(test)]
mod tests {
    use super::*;

    #[actix_rt::test]
    async fn polling_disable() {
        let setup = crate::tests::TestEnvironment::build().disable_polling().finish();
        let mut shared_state = setup.gen_shared_state();

        let machine = StateMachine::EntryPoint(State(EntryPoint {}))
            .move_to_next_state(&mut shared_state)
            .await
            .unwrap()
            .0;

        assert_state!(machine, Park);
    }

    #[actix_rt::test]
    async fn polling_enabled() {
        let setup = crate::tests::TestEnvironment::build().finish();
        let mut shared_state = setup.gen_shared_state();

        let machine = StateMachine::EntryPoint(State(EntryPoint {}))
            .move_to_next_state(&mut shared_state)
            .await
            .unwrap()
            .0;

        assert_state!(machine, Poll);
    }

    #[actix_rt::test]
    async fn forced_probe() {
        let setup = crate::tests::TestEnvironment::build().finish();
        let mut shared_state = setup.gen_shared_state();
        shared_state.runtime_settings.reset_installation_settings().unwrap();

        let (machine, trans) = StateMachine::EntryPoint(State(EntryPoint {}))
            .move_to_next_state(&mut shared_state)
            .await
            .unwrap();

        assert_state!(machine, Probe);
        match trans {
            actor::StepTransition::Immediate => {}
            _ => panic!("Unexpected StepTransition: {:?}", trans),
        }
    }
}
