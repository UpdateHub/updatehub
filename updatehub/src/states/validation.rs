// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    machine::{self, Context},
    Download, EntryPoint, Result, State, StateChangeImpl,
};
use crate::{object, update_package::UpdatePackageExt};
use slog_scope::{debug, error, info};

#[derive(Debug)]
pub(super) struct Validation {
    pub(super) package: cloud::api::UpdatePackage,
    pub(super) sign: Option<cloud::api::Signature>,
}

/// Implements the state change for State<Validation>.
#[async_trait::async_trait(?Send)]
impl StateChangeImpl for Validation {
    fn name(&self) -> &'static str {
        "validation"
    }

    fn is_preemptive_state(&self) -> bool {
        true
    }

    async fn handle(self, context: &mut Context) -> Result<(State, machine::StepTransition)> {
        if let Some(key) = context.firmware.pub_key.as_ref() {
            match self.sign.as_ref() {
                Some(sign) => {
                    debug!("validating signature");
                    sign.validate(key, &self.package)?;
                }
                None => {
                    error!("missing signature key");
                    return Err(super::TransitionError::SignatureNotFound);
                }
            }
        } else {
            info!("no signature key available on device, ignoring signature validation");
        }

        // Ensure the package is compatible
        let inactive_installation_set = context.runtime_settings.get_inactive_installation_set()?;
        self.package.compatible_with(&context.firmware)?;
        self.package.validate_install_modes(&context.settings, inactive_installation_set)?;
        self.package
            .objects(inactive_installation_set)
            .iter()
            .try_for_each(object::Installer::check_requirements)?;

        if context
            .runtime_settings
            .applied_package_uid()
            .map(|u| *u == self.package.package_uid())
            .unwrap_or_default()
        {
            info!("not downloading update package, the same package has already been installed");
            Ok((State::EntryPoint(EntryPoint {}), machine::StepTransition::Immediate))
        } else {
            Ok((
                State::Download(Download { update_package: self.package }),
                machine::StepTransition::Immediate,
            ))
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::{states::TransitionError, update_package::tests::get_update_package};

    #[async_std::test]
    async fn normal_transition() {
        let setup = crate::tests::TestEnvironment::build().finish();
        let mut context = setup.gen_context();
        let package = get_update_package();
        let sign = None;

        let machine = State::Validation(Validation { package, sign })
            .move_to_next_state(&mut context)
            .await
            .unwrap()
            .0;
        assert_state!(machine, Download);
    }

    #[async_std::test]
    async fn invalid_hardware() {
        let setup = crate::tests::TestEnvironment::build().invalid_hardware().finish();
        let mut context = setup.gen_context();
        let package = get_update_package();
        let sign = None;

        let machine =
            State::Validation(Validation { package, sign }).move_to_next_state(&mut context).await;

        match machine {
            Err(TransitionError::UpdatePackage(_)) => {}
            res => panic!("Unexpected result from transition: {:?}", res),
        }
    }

    #[async_std::test]
    async fn skip_same_package_uid() {
        let setup = crate::tests::TestEnvironment::build().finish();
        let mut context = setup.gen_context();
        let package = get_update_package();
        let sign = None;
        context.runtime_settings.set_applied_package_uid(&package.package_uid()).unwrap();

        let machine = State::Validation(Validation { package, sign })
            .move_to_next_state(&mut context)
            .await
            .unwrap()
            .0;
        assert_state!(machine, EntryPoint);
    }

    #[async_std::test]
    async fn missing_signature() {
        let setup = crate::tests::TestEnvironment::build().finish();
        let mut context = setup.gen_context();
        context.firmware.pub_key = Some("foo".into());

        let package = get_update_package();
        let sign = None;
        context.runtime_settings.set_applied_package_uid(&package.package_uid()).unwrap();

        let res =
            State::Validation(Validation { package, sign }).move_to_next_state(&mut context).await;
        match res {
            Err(crate::states::TransitionError::SignatureNotFound) => {}
            Err(e) => panic!("Unexpected error returned: {}", e),
            Ok(_) => panic!("Unexpected ok result returned"),
        }
    }
}
