// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    actor::{self, SharedState},
    EntryPoint, PrepareDownload, Result, State, StateChangeImpl,
};
use crate::update_package::UpdatePackageExt;
use slog_scope::{debug, error, info, trace};

#[derive(Debug, PartialEq)]
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

    async fn handle(
        self,
        shared_state: &mut SharedState,
    ) -> Result<(State, actor::StepTransition)> {
        if let Some(key) = shared_state.firmware.pub_key.as_ref() {
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
        }

        // Ensure the package is compatible
        self.package.compatible_with(&shared_state.firmware)?;

        if shared_state
            .runtime_settings
            .applied_package_uid()
            .map(|u| *u == self.package.package_uid())
            .unwrap_or_default()
        {
            info!("not downloading update package, the same package has already been installed.");
            Ok((State::EntryPoint(EntryPoint {}), actor::StepTransition::Immediate))
        } else {
            trace!("moving to PrepareDownload state to process the update package.");
            Ok((
                State::PrepareDownload(PrepareDownload { update_package: self.package }),
                actor::StepTransition::Immediate,
            ))
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::{states::TransitionError, update_package::tests::get_update_package};

    #[actix_rt::test]
    async fn normal_transition() {
        let setup = crate::tests::TestEnvironment::build().finish();
        let mut shared_state = setup.gen_shared_state();
        let package = get_update_package();
        let sign = None;

        let machine = State::Validation(Validation { package, sign })
            .move_to_next_state(&mut shared_state)
            .await
            .unwrap()
            .0;
        assert_state!(machine, PrepareDownload);
    }

    #[actix_rt::test]
    async fn invalid_hardware() {
        let setup = crate::tests::TestEnvironment::build().invalid_hardware().finish();
        let mut shared_state = setup.gen_shared_state();
        let package = get_update_package();
        let sign = None;

        let machine = State::Validation(Validation { package, sign })
            .move_to_next_state(&mut shared_state)
            .await;

        match machine {
            Err(TransitionError::UpdatePackage(_)) => {}
            res => panic!("Unexpected result from transition: {:?}", res),
        }
    }

    #[actix_rt::test]
    async fn skip_same_package_uid() {
        let setup = crate::tests::TestEnvironment::build().finish();
        let mut shared_state = setup.gen_shared_state();
        let package = get_update_package();
        let sign = None;
        shared_state.runtime_settings.set_applied_package_uid(&package.package_uid()).unwrap();

        let machine = State::Validation(Validation { package, sign })
            .move_to_next_state(&mut shared_state)
            .await
            .unwrap()
            .0;
        assert_state!(machine, EntryPoint);
    }

    #[actix_rt::test]
    async fn missing_signature() {
        let setup = crate::tests::TestEnvironment::build().finish();
        let mut shared_state = setup.gen_shared_state();
        shared_state.firmware.pub_key = Some("foo".into());

        let package = get_update_package();
        let sign = None;
        shared_state.runtime_settings.set_applied_package_uid(&package.package_uid()).unwrap();

        let res = State::Validation(Validation { package, sign })
            .move_to_next_state(&mut shared_state)
            .await;
        match res {
            Err(crate::states::TransitionError::SignatureNotFound) => {}
            Err(e) => panic!("Unexpected error returned: {}", e),
            Ok(_) => panic!("Unexpected ok result returned"),
        }
    }
}
