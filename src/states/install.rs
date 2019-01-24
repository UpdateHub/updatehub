// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::{
    states::{
        Idle, ProgressReporter, Reboot, State, StateChangeImpl, StateMachine, TransitionCallback,
    },
    update_package::UpdatePackage,
};

use slog::slog_info;
use slog_scope::info;

#[derive(Debug, PartialEq)]
pub(super) struct Install {
    pub(super) update_package: UpdatePackage,
}

create_state_step!(Install => Idle);
create_state_step!(Install => Reboot(update_package));

impl TransitionCallback for State<Install> {
    fn callback_state_name(&self) -> &'static str {
        "install"
    }
}

impl ProgressReporter for State<Install> {
    fn package_uid(&self) -> String {
        self.state.update_package.package_uid()
    }

    fn report_enter_state_name(&self) -> &'static str {
        "installing"
    }

    fn report_leave_state_name(&self) -> &'static str {
        "installed"
    }
}

impl StateChangeImpl for State<Install> {
    fn handle(mut self) -> Result<StateMachine, failure::Error> {
        let package_uid = self.state.update_package.package_uid();
        info!("Installing update: {}", &package_uid);

        // FIXME: Check if A/B install
        // FIXME: Check InstallIfDifferent

        // Ensure we do a probe as soon as possible so full update
        // cycle can be finished.
        self.runtime_settings.force_poll()?;

        // Avoid installing same package twice.
        self.runtime_settings
            .set_applied_package_uid(&package_uid)?;

        info!("Update installed successfully");
        let buffer = crate::logger::buffer();
        buffer.lock().unwrap().stop_logging();
        buffer.lock().unwrap().clear();
        Ok(StateMachine::Reboot(self.into()))
    }
}

#[cfg(test)]
mod test {
    use super::*;
    use crate::{
        firmware::Metadata, runtime_settings::RuntimeSettings, settings::Settings,
        update_package::tests::get_update_package,
    };
    use std::fs;
    use tempfile::NamedTempFile;

    fn fake_install_state() -> State<Install> {
        use crate::firmware::tests::{create_fake_metadata, FakeDevice};

        let tmpfile = NamedTempFile::new().unwrap();
        let tmpfile = tmpfile.path();
        fs::remove_file(&tmpfile).unwrap();
        State {
            settings: Settings::default(),
            runtime_settings: RuntimeSettings::new()
                .load(tmpfile.to_str().unwrap())
                .unwrap(),
            firmware: Metadata::from_path(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap(),
            state: Install {
                update_package: get_update_package(),
            },
        }
    }

    #[test]
    fn has_package_uid_if_succeed() {
        crate::logger::init(0);
        let machine = StateMachine::Install(fake_install_state()).move_to_next_state();

        match machine {
            Ok(StateMachine::Reboot(s)) => assert_eq!(
                s.runtime_settings.applied_package_uid(),
                Some(get_update_package().package_uid())
            ),
            Ok(s) => panic!("Invalid success: {:?}", s),
            Err(e) => panic!("Invalid error: {:?}", e),
        }
    }

    #[test]
    fn polling_now_if_succeed() {
        crate::logger::init(0);
        let machine = StateMachine::Install(fake_install_state()).move_to_next_state();

        match machine {
            Ok(StateMachine::Reboot(s)) => assert_eq!(s.runtime_settings.is_polling_forced(), true),
            Ok(s) => panic!("Invalid success: {:?}", s),
            Err(e) => panic!("Invalid error: {:?}", e),
        }
    }

    #[test]
    fn install_has_transition_callback_trait() {
        crate::logger::init(0);
        let state = fake_install_state();
        assert_eq!(state.callback_state_name(), "install");
    }
}
