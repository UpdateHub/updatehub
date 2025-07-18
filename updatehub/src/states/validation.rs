// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    Download, EntryPoint, Result, State, StateChangeImpl, TransitionError,
    install::Install,
    machine::{self, Context},
};
use crate::{
    object::{self, Info, Installer},
    update_package::UpdatePackageExt,
    utils::log::LogContent,
};
use slog_scope::{debug, error, info};

#[derive(Debug)]
pub(super) struct Validation {
    pub(super) package: cloud::api::UpdatePackage,
    pub(super) sign: Option<cloud::api::Signature>,
    pub(super) require_download: bool,
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
                    sign.validate(key, &self.package)
                        .log_error_msg("uhupkg failed signature validation")?;
                }
                None => {
                    error!("missing signature key");
                    return Err(super::TransitionError::SignatureNotFound);
                }
            }
        } else {
            info!("no signature key available on device, ignoring signature validation");
        }

        let object_context = object::installer::Context {
            download_dir: context.settings.update.download_dir.clone(),
            offline_update: !self.require_download,
            base_url: format!(
                "{server_url}/products/{product_uid}/packages/{package_uid}/objects",
                server_url = &context.server_address(),
                product_uid = &context.firmware.product_uid,
                package_uid = &self.package.package_uid(),
            ),
        };

        // Ensure the package is compatible
        let inactive_installation_set = context
            .runtime_settings
            .get_inactive_installation_set()
            .log_error_msg("unable to get inactive installation set")?;
        self.package
            .compatible_with(&context.firmware)
            .log_error_msg("uhupkg is not compatible with this device")?;
        self.package
            .validate_install_modes(&context.settings, inactive_installation_set)
            .log_error_msg("install mode failed validation")?;
        for obj in self.package.objects(inactive_installation_set).iter() {
            if let Err(e) = obj.check_requirements(&object_context).await {
                error!(
                    "update package: {} ({}) has failed to meet the install requirements",
                    self.package.version(),
                    self.package.package_uid()
                );
                return Err(e.into());
            }
        }

        let update_package = self.package.clone();
        let sign = self.sign.clone();

        if context
            .runtime_settings
            .applied_package_uid()
            .map(|u| *u == update_package.package_uid())
            .unwrap_or_default()
        {
            info!("not downloading update package, the same package has already been installed");
            Ok((State::EntryPoint(EntryPoint {}), machine::StepTransition::Immediate))
        } else {
            let next_state = if self.require_download {
                State::Download(Download { update_package, sign })
            } else {
                // Ensure all objects are Ready for use
                let download_dir = &context.settings.update.download_dir;
                let not_ready: Vec<_> = update_package
                    .objects(inactive_installation_set)
                    .iter()
                    .filter(|o| !o.allow_remote_install())
                    .filter_map(|o| match (o.filename(), o.status(download_dir)) {
                        (_, Ok(object::info::Status::Ready)) => None,
                        (filename, status) => Some((filename, status)),
                    })
                    .collect();

                if not_ready.is_empty() {
                    State::Install(Install { update_package, object_context })
                } else {
                    error!("some objects are not ready for use:");
                    for object in not_ready {
                        match object {
                            (filename, Ok(status)) => {
                                error!(" file '{}' is {:?}", filename, status)
                            }
                            (filename, Err(err)) => {
                                error!(" file '{}' has failed with error: {:?}", filename, err)
                            }
                        }
                    }
                    return Err(TransitionError::SomeObjectsAreNotReady);
                }
            };

            Ok((next_state, machine::StepTransition::Immediate))
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::{states::TransitionError, update_package::tests::get_update_package};

    #[tokio::test]
    async fn normal_transition() {
        let setup = crate::tests::TestEnvironment::build().finish();
        let mut context = setup.gen_context();
        let package = get_update_package();
        let sign = None;

        let machine = State::Validation(Validation { package, sign, require_download: true })
            .move_to_next_state(&mut context)
            .await
            .unwrap()
            .0;
        assert_state!(machine, Download);
    }

    #[tokio::test]
    async fn invalid_hardware() {
        let setup = crate::tests::TestEnvironment::build().invalid_hardware().finish();
        let mut context = setup.gen_context();
        let package = get_update_package();
        let sign = None;

        let machine = State::Validation(Validation { package, sign, require_download: true })
            .move_to_next_state(&mut context)
            .await;

        match machine {
            Err(TransitionError::UpdatePackage(_)) => {}
            res => panic!("Unexpected result from transition: {res:?}"),
        }
    }

    #[tokio::test]
    async fn skip_same_package_uid() {
        let setup = crate::tests::TestEnvironment::build().finish();
        let mut context = setup.gen_context();
        let package = get_update_package();
        let sign = None;
        context.runtime_settings.set_applied_package_uid(&package.package_uid()).unwrap();

        let machine = State::Validation(Validation { package, sign, require_download: true })
            .move_to_next_state(&mut context)
            .await
            .unwrap()
            .0;
        assert_state!(machine, EntryPoint);
    }

    #[tokio::test]
    async fn missing_signature() {
        let setup = crate::tests::TestEnvironment::build().finish();
        let mut context = setup.gen_context();
        context.firmware.pub_key = Some("foo".into());

        let package = get_update_package();
        let sign = None;
        context.runtime_settings.set_applied_package_uid(&package.package_uid()).unwrap();

        let res = State::Validation(Validation { package, sign, require_download: true })
            .move_to_next_state(&mut context)
            .await;
        match res {
            Err(crate::states::TransitionError::SignatureNotFound) => {}
            Err(e) => panic!("Unexpected error returned: {e}"),
            Ok(_) => panic!("Unexpected ok result returned"),
        }
    }
}
