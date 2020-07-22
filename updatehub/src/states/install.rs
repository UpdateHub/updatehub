// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    machine::{self, Context},
    ProgressReporter, Reboot, Result, State, StateChangeImpl,
};
use crate::{
    firmware::installation_set,
    object::{self, Installer},
    update_package::{UpdatePackage, UpdatePackageExt},
};
use slog_scope::{debug, info};

#[derive(Debug, PartialEq)]
pub(super) struct Install {
    pub(super) update_package: UpdatePackage,
}

impl ProgressReporter for Install {
    fn package_uid(&self) -> String {
        self.update_package.package_uid()
    }

    fn report_enter_state_name(&self) -> &'static str {
        "installing"
    }

    fn report_leave_state_name(&self) -> &'static str {
        "installed"
    }
}

pub(crate) trait ObjectInstaller {
    fn check_requirements(&self) -> crate::Result<()> {
        debug!("running default check_requirements");
        Ok(())
    }

    fn setup(&mut self) -> crate::Result<()> {
        debug!("running default setup");
        Ok(())
    }

    fn cleanup(&mut self) -> crate::Result<()> {
        debug!("running default cleanup");
        Ok(())
    }

    fn install(&self, download_dir: std::path::PathBuf) -> crate::Result<()>;
}

#[async_trait::async_trait(?Send)]
impl StateChangeImpl for Install {
    fn name(&self) -> &'static str {
        "install"
    }

    async fn handle(mut self, context: &mut Context) -> Result<(State, machine::StepTransition)> {
        let package_uid = self.update_package.package_uid();
        info!("installing update: {}", &package_uid);

        let installation_set = context.runtime_settings.get_inactive_installation_set()?;
        info!("using installation set as target {}", installation_set);

        // FIXME: What is missing:
        //
        // - verify if the object needs to be installed, accordingly to the install if
        //   different rule.

        let objs = self.update_package.objects_mut(installation_set);
        objs.iter().try_for_each(object::Installer::check_requirements)?;
        objs.iter_mut().try_for_each(object::Installer::setup)?;
        objs.iter_mut().try_for_each(|obj| {
            obj.install(&context.settings.update.download_dir)?;
            obj.cleanup()
        })?;

        // Avoid installing same package twice.
        context.runtime_settings.set_applied_package_uid(&package_uid)?;

        // Set upgrading to the new installation set
        context.runtime_settings.set_upgrading_to(installation_set)?;

        // Swap installation set so it is used next device boot.
        installation_set::swap_active()?;
        info!("swapping active installation set");

        info!("update installed successfully");
        Ok((
            State::Reboot(Reboot { update_package: self.update_package }),
            machine::StepTransition::Immediate,
        ))
    }
}

#[cfg(test)]
mod test {
    use super::*;
    use crate::update_package::tests::get_update_package;
    use pretty_assertions::assert_eq;

    #[async_std::test]
    async fn has_package_uid_if_succeed() {
        let setup = crate::tests::TestEnvironment::build().finish();
        let mut context = setup.gen_context();
        let state = Install { update_package: get_update_package() };

        let machine = State::Install(state).move_to_next_state(&mut context).await.unwrap().0;

        match machine {
            State::Reboot(_) => assert_eq!(
                context.runtime_settings.applied_package_uid(),
                Some(get_update_package().package_uid())
            ),
            s => panic!("Invalid success: {:?}", s),
        }
    }
}
