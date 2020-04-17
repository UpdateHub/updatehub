// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    actor::{self, SharedState},
    EntryPoint, PrepareDownload, Result, State, StateChangeImpl, StateMachine,
};
use crate::update_package::UpdatePackageExt;
use slog_scope::{debug, error, info};

#[derive(Debug, PartialEq)]
pub(super) struct Validation {
    pub(super) package: cloud::api::UpdatePackage,
    pub(super) sign: Option<cloud::api::Signature>,
}

create_state_step!(Validation => EntryPoint);

/// Implements the state change for State<Validation>.
#[async_trait::async_trait(?Send)]
impl StateChangeImpl for State<Validation> {
    fn name(&self) -> &'static str {
        "validation"
    }

    fn can_run_trigger_probe(&self) -> bool {
        true
    }

    async fn handle(
        self,
        shared_state: &mut SharedState,
    ) -> Result<(StateMachine, actor::StepTransition)> {
        if let Some(key) = shared_state.firmware.pub_key.as_ref() {
            match self.0.sign.as_ref() {
                Some(sign) => {
                    debug!("Validating signature");
                    sign.validate(key, &self.0.package)?;
                }
                None => {
                    error!("Missing signature key");
                    return Err(super::TransitionError::SignatureNotFound);
                }
            }
        }

        // Ensure the package is compatible
        self.0.package.compatible_with(&shared_state.firmware)?;

        if shared_state
            .runtime_settings
            .applied_package_uid()
            .map(|u| *u == self.0.package.package_uid())
            .unwrap_or_default()
        {
            info!("Not downloading update package. Same package has already been installed.");
            debug!("Moving to EntryPoint as this update package is already installed.");
            Ok((StateMachine::EntryPoint(self.into()), actor::StepTransition::Immediate))
        } else {
            debug!("Moving to PrepareDownload state to process the update package.");
            Ok((
                StateMachine::PrepareDownload(State(PrepareDownload {
                    update_package: self.0.package,
                })),
                actor::StepTransition::Immediate,
            ))
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::{
        firmware::{
            tests::{create_fake_metadata, FakeDevice},
            Metadata,
        },
        runtime_settings::RuntimeSettings,
        settings::Settings,
        states::TransitionError,
        update_package::tests::get_update_package,
    };
    use tempfile::NamedTempFile;

    #[actix_rt::test]
    async fn normal_transition() {
        let tmpfile = NamedTempFile::new().unwrap();
        let tmpfile = tmpfile.path();
        std::fs::remove_file(&tmpfile).unwrap();

        let settings = Settings::default();
        let runtime_settings = RuntimeSettings::load(tmpfile).unwrap();
        let firmware = Metadata::from_path(&create_fake_metadata(FakeDevice::HasUpdate)).unwrap();
        let mut shared_state = SharedState { settings, runtime_settings, firmware };
        let package = get_update_package();
        let sign = None;

        let machine = StateMachine::Validation(State(Validation { package, sign }))
            .move_to_next_state(&mut shared_state)
            .await
            .unwrap()
            .0;
        assert_state!(machine, PrepareDownload);
    }

    #[actix_rt::test]
    async fn invalid_hardware() {
        let tmpfile = NamedTempFile::new().unwrap();
        let tmpfile = tmpfile.path();
        std::fs::remove_file(&tmpfile).unwrap();

        let settings = Settings::default();
        let runtime_settings = RuntimeSettings::load(tmpfile).unwrap();
        let firmware =
            Metadata::from_path(&create_fake_metadata(FakeDevice::InvalidHardware)).unwrap();
        let mut shared_state = SharedState { settings, runtime_settings, firmware };
        let package = get_update_package();
        let sign = None;

        let machine = StateMachine::Validation(State(Validation { package, sign }))
            .move_to_next_state(&mut shared_state)
            .await;

        match machine {
            Err(TransitionError::UpdatePackage(_)) => {}
            res => panic!("Unexpected result from transition: {:?}", res),
        }
    }

    #[actix_rt::test]
    async fn skip_same_package_uid() {
        let tmpfile = NamedTempFile::new().unwrap();
        let tmpfile = tmpfile.path();
        std::fs::remove_file(&tmpfile).unwrap();

        let runtime_settings = RuntimeSettings::load(tmpfile).unwrap();
        let settings = Settings::default();
        let firmware = Metadata::from_path(&create_fake_metadata(FakeDevice::HasUpdate)).unwrap();
        let mut shared_state = SharedState { settings, runtime_settings, firmware };

        let package = get_update_package();
        let sign = None;
        shared_state.runtime_settings.set_applied_package_uid(&package.package_uid()).unwrap();

        let machine = StateMachine::Validation(State(Validation { package, sign }))
            .move_to_next_state(&mut shared_state)
            .await
            .unwrap()
            .0;
        assert_state!(machine, EntryPoint);
    }

    #[actix_rt::test]
    async fn missing_signature() {
        let tmpfile = NamedTempFile::new().unwrap();
        let tmpfile = tmpfile.path();
        std::fs::remove_file(&tmpfile).unwrap();

        let runtime_settings = RuntimeSettings::load(tmpfile).unwrap();
        let settings = Settings::default();
        let firmware = Metadata::from_path(&create_fake_metadata(FakeDevice::HasUpdate)).unwrap();
        let mut shared_state = SharedState { settings, runtime_settings, firmware };
        shared_state.firmware.pub_key = Some("foo".into());

        let package = get_update_package();
        let sign = None;
        shared_state.runtime_settings.set_applied_package_uid(&package.package_uid()).unwrap();

        let res = StateMachine::Validation(State(Validation { package, sign }))
            .move_to_next_state(&mut shared_state)
            .await;
        match res {
            Err(crate::states::TransitionError::SignatureNotFound) => {}
            Err(e) => panic!("Unexpected error returned: {}", e),
            Ok(_) => panic!("Unexpected ok result returned"),
        }
    }
}
