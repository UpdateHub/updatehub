// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    CallbackReporter, ProgressReporter, Reboot, Result, State, StateChangeImpl,
    machine::{self, Context},
};
use crate::{
    firmware::{boot_emmc, installation_set},
    object::{self, Info, Installer},
    update_package::{UpdatePackage, UpdatePackageExt},
    utils::log::LogContent,
};
use slog_scope::{error, info, warn};

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
        // The objects write the boot partition picked when the package was
        // validated, which must still be the one prepared here
        if let Some(target) = &obj_context.boot_emmc {
            if target.partition != installation_set.index() {
                return Err(crate::firmware::Error::EmmcTargetChanged {
                    validated: target.partition,
                    installing: installation_set.index(),
                }
                .into());
            }
        }
        let objs = self.update_package.objects_mut(installation_set);

        // Objects are sorted in reverse order so the smaller objects are installed
        // later. This postpones objects like U-Boot updates and U-Boot environment
        // changes towards the end of the update.
        objs.sort_by(|a, b| a.len().partial_cmp(&b.len()).unwrap().reverse());

        boot_emmc::prepare_install(
            &context.settings,
            !context.settings.storage.read_only,
            context.runtime_settings.update.upgrade_to_installation.map(installation_set::Set),
            installation_set.index(),
        )
        .log_error_msg("unable to prepare the eMMC boot partitions")?;

        // Run the install routine for every object.
        for obj in objs.iter_mut() {
            obj.install(&obj_context).await?;
        }

        boot_emmc::check_installed(&context.settings, installation_set.index())
            .log_error_msg("unable to install the eMMC boot partition")?;

        // Avoid installing same package twice.
        context
            .runtime_settings
            .set_applied_package_uid(&package_uid)
            .log_error_msg("failed to set applied package uid to runtime settings")?;

        // Set upgrading to the new installation set.
        context
            .runtime_settings
            .set_upgrading_to(installation_set, boot_emmc::emmc(&context.settings).is_some())
            .log_error_msg("failed to upgrade installation set to runtime settings ")?;

        // Swap installation set so it is used next device boot.
        info!("swapping active installation set");
        // A swap that fails without changing the set leaves no update to
        // wait for
        if let Err(e) = installation_set::swap_active() {
            error!("unable to update active installation set: {}", e);
            if installation_set::active()? != installation_set {
                context
                    .runtime_settings
                    .clear_upgrading()
                    .log_error_msg("failed to reset the installation settings")?;
                return Err(e.into());
            }
            warn!("the active installation set changed nonetheless");
        }

        // When the eMMC cannot follow the new set, the set goes back, so
        // firmware and system stay a pair. Should the eMMC not follow back
        // either, the update stays pending, and the next start makes it.
        if let Err(e) = boot_emmc::follow_active_set(&context.settings, "update installed") {
            error!("unable to boot the eMMC from the new installation set: {}", e);
            installation_set::swap_active()
                .log_error_msg("unable to restore the active installation set")?;
            match boot_emmc::follow_active_set(&context.settings, "update aborted") {
                Ok(_) => context
                    .runtime_settings
                    .clear_upgrading()
                    .log_error_msg("failed to reset the installation settings")?,
                Err(e) => {
                    error!("unable to boot the eMMC from the previous installation set: {}", e)
                }
            }
            return Err(e.into());
        }

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
            s => panic!("Invalid success: {s:?}"),
        }
    }
}
