// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    actor::{self, SharedState},
    Idle, ProgressReporter, Reboot, State, StateChangeImpl, StateMachine, TransitionCallback,
};
use crate::{
    firmware::installation_set,
    object::{self, Installer},
    update_package::UpdatePackage,
};
use slog_scope::{debug, info};

#[derive(Debug, PartialEq)]
pub(super) struct Install {
    pub(super) update_package: UpdatePackage,
}

create_state_step!(Install => Idle);
create_state_step!(Install => Reboot(update_package));

impl TransitionCallback for State<Install> {}

impl ProgressReporter for State<Install> {
    fn package_uid(&self) -> String {
        self.0.update_package.package_uid()
    }

    fn report_enter_state_name(&self) -> &'static str {
        "installing"
    }

    fn report_leave_state_name(&self) -> &'static str {
        "installed"
    }
}

pub(crate) trait ObjectInstaller {
    fn check_requirements(&self) -> Result<(), failure::Error> {
        debug!("running default check_requirements");
        Ok(())
    }

    fn setup(&mut self) -> Result<(), failure::Error> {
        debug!("running default setup");
        Ok(())
    }

    fn cleanup(&mut self) -> Result<(), failure::Error> {
        debug!("running default cleanup");
        Ok(())
    }

    fn install(&self, download_dir: std::path::PathBuf) -> Result<(), failure::Error>;
}

#[async_trait::async_trait]
impl StateChangeImpl for State<Install> {
    fn name(&self) -> &'static str {
        "install"
    }

    async fn handle(
        mut self,
        shared_state: &mut SharedState,
    ) -> Result<(StateMachine, actor::StepTransition), failure::Error> {
        let package_uid = self.0.update_package.package_uid();
        info!("Installing update: {}", &package_uid);

        let installation_set = installation_set::inactive()?;
        info!("Using installation set as target {}", installation_set);

        // FIXME: What is missing:
        //
        // - verify if the object needs to be installed, accordingly to the install if
        //   different rule.

        let objs = self.0.update_package.objects_mut(installation_set);
        objs.iter().try_for_each(object::Installer::check_requirements)?;
        objs.iter_mut().try_for_each(object::Installer::setup)?;
        objs.iter_mut().try_for_each(|obj| {
            obj.install(&shared_state.settings.update.download_dir)?;
            obj.cleanup()
        })?;

        // Ensure we do a probe as soon as possible so full update
        // cycle can be finished.
        shared_state.runtime_settings.force_poll()?;

        // Avoid installing same package twice.
        shared_state.runtime_settings.set_applied_package_uid(&package_uid)?;

        // Swap installation set so it is used next device boot.
        installation_set::swap_active()?;
        info!("Swapping active installation set");

        info!("Update installed successfully");
        let buffer = crate::logger::buffer();
        buffer.lock().unwrap().stop_logging();
        buffer.lock().unwrap().clear();
        Ok((StateMachine::Reboot(self.into()), actor::StepTransition::Immediate))
    }
}

#[cfg(test)]
mod test {
    use super::*;
    use crate::{
        firmware::Metadata, runtime_settings::RuntimeSettings,
        update_package::tests::get_update_package,
    };
    use pretty_assertions::assert_eq;
    use std::fs;
    use tempfile::NamedTempFile;

    fn fake_install_state() -> (State<Install>, SharedState) {
        use crate::{
            firmware::tests::{create_fake_installation_set, create_fake_metadata, FakeDevice},
            update_package::tests::create_fake_settings,
        };
        use std::env;

        let tmpfile = NamedTempFile::new().unwrap();
        let tmpfile = tmpfile.path();
        fs::remove_file(&tmpfile).unwrap();

        let settings = create_fake_settings();
        let tmpdir = settings.update.download_dir.clone();
        create_fake_installation_set(&tmpdir, 0);
        env::set_var("PATH", format!("{}", &tmpdir.to_string_lossy()));

        let runtime_settings = RuntimeSettings::default();
        let firmware = Metadata::from_path(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap();
        let shared_state = SharedState { settings, runtime_settings, firmware };

        (State(Install { update_package: get_update_package() }), shared_state)
    }

    #[actix_rt::test]
    async fn has_package_uid_if_succeed() {
        let (state, mut shared_state) = fake_install_state();
        let machine =
            StateMachine::Install(state).move_to_next_state(&mut shared_state).await.unwrap().0;

        match machine {
            StateMachine::Reboot(_) => assert_eq!(
                shared_state.runtime_settings.applied_package_uid(),
                Some(get_update_package().package_uid())
            ),
            s => panic!("Invalid success: {:?}", s),
        }
    }

    #[actix_rt::test]
    async fn polling_now_if_succeed() {
        let (state, mut shared_state) = fake_install_state();
        let machine =
            StateMachine::Install(state).move_to_next_state(&mut shared_state).await.unwrap().0;

        match machine {
            StateMachine::Reboot(_) => {
                assert_eq!(shared_state.runtime_settings.is_polling_forced(), true)
            }
            s => panic!("Invalid success: {:?}", s),
        }
    }
}
