// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
//

use failure::{Error, ResultExt};
use states::{Idle, Reboot, State, StateChangeImpl, StateMachine};
use update_package::UpdatePackage;

#[derive(Debug, PartialEq)]
pub struct Install {
    pub update_package: UpdatePackage,
}

create_state_step!(Install => Idle);
create_state_step!(Install => Reboot);

impl StateChangeImpl for State<Install> {
    // FIXME: When adding state-chance hooks, we need to go to Idle if
    // cancelled.
    fn to_next_state(mut self) -> Result<StateMachine, Error> {
        info!(
            "Installing update: {}",
            self.state.update_package.package_uid()
        );

        // FIXME: Check if A/B install
        // FIXME: Check InstallIfDifferent

        // Ensure we do a probe as soon as possible so full update
        // cycle can be finished.
        self.runtime_settings.polling.now = true;

        // Avoid installing same package twice.
        self.applied_package_uid = Some(self.state.update_package.package_uid());

        if !self.settings.storage.read_only {
            debug!("Saving install settings.");
            self.runtime_settings
                .save()
                .context("Saving runtime due install changes")?;
        } else {
            debug!("Skipping install settings save, read-only mode enabled.");
        }

        info!("Update installed successfully");
        Ok(StateMachine::Reboot(self.into()))
    }
}

#[test]
fn has_package_uid_if_succeed() {
    use super::*;
    use firmware::tests::{create_fake_metadata, FakeDevice};
    use mktemp::Temp;
    use update_package::tests::get_update_package;

    let tmpfile = Temp::new_file().unwrap().to_path_buf();

    let machine = StateMachine::Install(State {
        settings: Settings::default(),
        runtime_settings: RuntimeSettings::new()
            .load(tmpfile.to_str().unwrap())
            .unwrap(),
        firmware: Metadata::new(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap(),
        applied_package_uid: None,
        state: Install {
            update_package: get_update_package(),
        },
    }).move_to_next_state();

    match machine {
        Ok(StateMachine::Reboot(s)) => assert_eq!(
            s.applied_package_uid,
            Some(get_update_package().package_uid())
        ),
        Ok(s) => panic!("Invalid success: {:?}", s),
        Err(e) => panic!("Invalid error: {:?}", e),
    }
}

#[test]
fn polling_now_if_succeed() {
    use super::*;
    use firmware::tests::{create_fake_metadata, FakeDevice};
    use mktemp::Temp;
    use update_package::tests::get_update_package;

    let tmpfile = Temp::new_file().unwrap().to_path_buf();

    let machine = StateMachine::Install(State {
        settings: Settings::default(),
        runtime_settings: RuntimeSettings::new()
            .load(tmpfile.to_str().unwrap())
            .unwrap(),
        firmware: Metadata::new(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap(),
        applied_package_uid: None,
        state: Install {
            update_package: get_update_package(),
        },
    }).move_to_next_state();

    match machine {
        Ok(StateMachine::Reboot(s)) => assert_eq!(s.runtime_settings.polling.now, true),
        Ok(s) => panic!("Invalid success: {:?}", s),
        Err(e) => panic!("Invalid error: {:?}", e),
    }
}
