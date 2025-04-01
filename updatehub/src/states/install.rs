// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    CallbackReporter, ProgressReporter, Reboot, Result, State, StateChangeImpl,
    machine::{self, Context},
};
use crate::{
    firmware::installation_set,
    object::{self, Info, Installer},
    update_package::{UpdatePackage, UpdatePackageExt},
    utils::log::LogContent,
};
use slog_scope::info;

#[derive(Debug)]
pub(super) struct Install {
    pub(super) update_package: UpdatePackage,
    pub(super) object_context: object::installer::Context,
}

impl CallbackReporter for Install {}

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

#[async_trait::async_trait(?Send)]
impl StateChangeImpl for Install {
    fn name(&self) -> &'static str {
        "install"
    }

    async fn handle(mut self, context: &mut Context) -> Result<(State, machine::StepTransition)> {
        let package_uid = self.update_package.package_uid();
        info!("installing update: {} ({})", self.update_package.version(), &package_uid);

        let installation_set = context
            .runtime_settings
            .get_inactive_installation_set()
            .log_error_msg("unable to get inactive installation set")?;
        info!("using installation set as target {}", installation_set);

        let obj_context = self.object_context;
        let objs = self.update_package.objects_mut(installation_set);

        // Objects are sorted in reverse order so the smaller objects are installed
        // later. This postpones objects like U-Boot updates and U-Boot environment
        // changes towards the end of the update.
        objs.sort_by(|a, b| a.len().partial_cmp(&b.len()).unwrap().reverse());

        // Run the install routine for every object.
        for obj in objs.iter_mut() {
            obj.install(&obj_context).await?;
        }

        // Avoid installing same package twice.
        context
            .runtime_settings
            .set_applied_package_uid(&package_uid)
            .log_error_msg("failed to set applied package uid to runtime settings")?;

        // Set upgrading to the new installation set.
        context
            .runtime_settings
            .set_upgrading_to(installation_set)
            .log_error_msg("failed to upgrade installation set to runtime settings ")?;

        // Swap installation set so it is used next device boot.
        info!("swapping active installation set");
        installation_set::swap_active()
            .log_error_msg("unable to update active installation set")?;

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

    #[tokio::test]
    async fn has_package_uid_if_succeed() {
        let setup = crate::tests::TestEnvironment::build().finish();
        let mut context = setup.gen_context();
        let state = Install {
            update_package: get_update_package(),
            object_context: object::installer::Context::default(),
        };

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
