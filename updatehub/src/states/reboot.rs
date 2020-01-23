// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    actor::{self, SharedState},
    Idle, ProgressReporter, State, StateChangeImpl, StateMachine, TransitionCallback,
};
use crate::update_package::UpdatePackage;

use easy_process;
use slog_scope::info;

#[derive(Debug, PartialEq)]
pub(super) struct Reboot {
    pub(super) update_package: UpdatePackage,
}

create_state_step!(Reboot => Idle);

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

#[async_trait::async_trait]
impl StateChangeImpl for State<Reboot> {
    fn name(&self) -> &'static str {
        "reboot"
    }

    async fn handle(
        self,
        _: &mut SharedState,
    ) -> Result<(StateMachine, actor::StepTransition), failure::Error> {
        info!("Triggering reboot");
        let output = easy_process::run("reboot")?;
        if !output.stdout.is_empty() || !output.stderr.is_empty() {
            info!("  reboot output: stdout: {}, stderr: {}", output.stdout, output.stderr);
        }
        Ok((StateMachine::Idle(self.into()), actor::StepTransition::Immediate))
    }
}

#[cfg(test)]
mod test {
    use super::*;
    use pretty_assertions::assert_eq;
    use std::path::Path;

    fn fake_reboot_state() -> (State<Reboot>, SharedState) {
        use crate::{
            firmware::{
                tests::{create_fake_metadata, FakeDevice},
                Metadata,
            },
            runtime_settings::RuntimeSettings,
            settings::Settings,
            update_package::tests::get_update_package,
        };

        let settings = Settings::default();
        let runtime_settings = RuntimeSettings::default();
        let firmware = Metadata::from_path(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap();
        let shared_state = SharedState { settings, runtime_settings, firmware };

        (State(Reboot { update_package: get_update_package() }), shared_state)
    }

    fn create_reboot(path: &Path) {
        use std::{
            fs::{create_dir_all, metadata, File},
            io::Write,
            os::unix::fs::PermissionsExt,
        };

        // ensure path exists
        create_dir_all(path).unwrap();

        let mut file = File::create(&path.join("reboot")).unwrap();
        writeln!(file, "#!/bin/sh\necho reboot").unwrap();

        let mut permissions = metadata(path).unwrap().permissions();
        permissions.set_mode(0o755);
        file.set_permissions(permissions).unwrap();
    }

    #[actix_rt::test]
    async fn runs() {
        use std::env;
        use tempfile::tempdir;

        // create the fake reboot command
        let tmpdir = tempdir().unwrap();
        let tmpdir = tmpdir.path();
        create_reboot(&tmpdir);
        env::set_var("PATH", format!("{}", &tmpdir.to_string_lossy()));

        let (state, mut shared_state) = fake_reboot_state();
        let machine =
            StateMachine::Reboot(state).move_to_next_state(&mut shared_state).await.unwrap().0;

        assert_state!(machine, Idle);
    }

    #[test]
    fn reboot_has_transition_callback_trait() {
        let (state, _) = fake_reboot_state();
        assert_eq!(state.name(), "reboot");
    }
}
