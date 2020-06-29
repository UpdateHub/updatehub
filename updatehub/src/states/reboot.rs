// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    actor::{self, SharedState},
    EntryPoint, ProgressReporter, Result, State, StateChangeImpl, StateMachine, TransitionCallback,
};
use crate::update_package::UpdatePackage;
use slog_scope::{info, warn};

#[derive(Debug, PartialEq)]
pub(super) struct Reboot {
    pub(super) update_package: UpdatePackage,
}

create_state_step!(Reboot => EntryPoint);

impl TransitionCallback for State<Reboot> {}

impl ProgressReporter for State<Reboot> {
    fn package_uid(&self) -> String {
        self.0.update_package.package_uid()
    }

    fn report_enter_state_name(&self) -> &'static str {
        "rebooting"
    }

    fn report_leave_state_name(&self) -> &'static str {
        "rebooting"
    }
}

#[async_trait::async_trait(?Send)]
impl StateChangeImpl for State<Reboot> {
    fn name(&self) -> &'static str {
        "reboot"
    }

    async fn handle(self, _: &mut SharedState) -> Result<(StateMachine, actor::StepTransition)> {
        info!("triggering reboot");
        let output = easy_process::run("reboot")?;
        if !output.stdout.is_empty() || !output.stderr.is_empty() {
            warn!("  reboot output: stdout: {}, stderr: {}", output.stdout, output.stderr);
        }
        Ok((StateMachine::EntryPoint(self.into()), actor::StepTransition::Immediate))
    }
}

#[cfg(test)]
mod test {
    use super::*;
    use crate::update_package::tests::get_update_package;
    use pretty_assertions::assert_eq;

    #[actix_rt::test]
    async fn runs() {
        let setup = crate::tests::TestEnvironment::build().add_echo_binary("reboot").finish();
        let mut shared_state = setup.gen_shared_state();
        let state = State(Reboot { update_package: get_update_package() });

        let machine =
            StateMachine::Reboot(state).move_to_next_state(&mut shared_state).await.unwrap().0;

        assert_state!(machine, EntryPoint);
    }

    #[test]
    fn reboot_has_transition_callback_trait() {
        let state = State(Reboot { update_package: get_update_package() });
        assert_eq!(state.name(), "reboot");
    }
}
