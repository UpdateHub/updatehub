// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

mod hook;
pub mod installation_set;

#[cfg(feature = "test-env")]
pub mod tests;

use self::hook::{run_hook, run_hooks_from_dir};
use derive_more::{Deref, DerefMut, Display, Error, From};
pub use sdk::api::info::firmware as api;
use slog_scope::{error, trace};
use std::{io, path::Path};

const PRODUCT_UID_HOOK: &str = "product-uid";
const VERSION_HOOK: &str = "version";
const HARDWARE_HOOK: &str = "hardware";
const PUB_KEY: &str = "key.pub";
const DEVICE_IDENTITY_DIR: &str = "device-identity.d";
const DEVICE_ATTRIBUTES_DIR: &str = "device-attributes.d";
const STATE_CHANGE_CALLBACK: &str = "state-change-callback";
const VALIDATE_CALLBACK: &str = "validate-callback";
const ROLLBACK_CALLBACK: &str = "rollback-callback";
const ERROR_CALLBACK: &str = "error-callback";

pub type Result<T> = std::result::Result<T, Error>;

#[derive(Debug, Display, Error, From)]
pub enum Error {
    #[display("invalid product UID")]
    InvalidProductUid,

    #[display("product UID is missing")]
    MissingProductUid,

    #[display("device identity is missing")]
    MissingDeviceIdentity,

    #[display(fmt = "{} is a invalid value. The only know ones are 0 or 1", _0)]
    InvalidInstallSet(#[error(not(source))] u8),

    ParseInt(std::num::ParseIntError),

    Walkdir(walkdir::Error),

    Io(std::io::Error),

    Process(easy_process::Error),
}

#[derive(Debug, PartialEq)]
pub(crate) enum Transition {
    Continue,
    Cancel,
}

#[derive(Clone, Debug, Deref, DerefMut, PartialEq)]
pub struct Metadata(pub api::Metadata);

impl Metadata {
    pub fn from_path(path: &Path) -> Result<Self> {
        let product_uid_hook = path.join(PRODUCT_UID_HOOK);
        let version_hook = path.join(VERSION_HOOK);
        let hardware_hook = path.join(HARDWARE_HOOK);
        let device_identity_dir = path.join(DEVICE_IDENTITY_DIR);
        let device_attributes_dir = path.join(DEVICE_ATTRIBUTES_DIR);
        let pub_key_path = path.join(PUB_KEY);

        let metadata = Metadata(api::Metadata {
            product_uid: run_hook(&product_uid_hook)?,
            version: run_hook(&version_hook)?,
            hardware: run_hook(&hardware_hook)?,
            pub_key: if pub_key_path.exists() { Some(pub_key_path) } else { None },
            device_identity: run_hooks_from_dir(&device_identity_dir)?,
            device_attributes: run_hooks_from_dir(&device_attributes_dir).unwrap_or_default(),
        });

        if metadata.product_uid.is_empty() {
            return Err(Error::MissingProductUid);
        }

        if metadata.product_uid.len() != 64 {
            return Err(Error::InvalidProductUid);
        }

        if metadata.device_identity.is_empty() {
            return Err(Error::MissingDeviceIdentity);
        }

        Ok(metadata)
    }

    pub(crate) fn as_cloud_metadata(&self) -> cloud::api::FirmwareMetadata<'_> {
        cloud::api::FirmwareMetadata {
            product_uid: &self.0.product_uid,
            version: &self.0.version,
            hardware: &self.0.hardware,
            device_identity: cloud::api::MetadataValue(&self.0.device_identity.0),
            device_attributes: cloud::api::MetadataValue(&self.0.device_attributes.0),
        }
    }
}

pub(crate) fn state_change_callback(path: &Path, state: &str) -> Result<Transition> {
    let callback = path.join(STATE_CHANGE_CALLBACK);
    if !callback.exists() {
        return Ok(Transition::Continue);
    }

    let output = easy_process::run(&format!("{} {}", &callback.to_string_lossy(), &state))?;
    for err in output.stderr.lines() {
        error!("{} (stderr): {}", path.display(), err);
    }

    match output.stdout.trim() {
        "cancel" => Ok(Transition::Cancel),
        "" => Ok(Transition::Continue),
        _ => Err(io::Error::new(
            io::ErrorKind::InvalidInput,
            format!(
                "invalid output format from 'state-change-callback' hook for state '{}'",
                &state
            ),
        )
        .into()),
    }
}

pub(crate) fn validate_callback(path: &Path) -> Result<Transition> {
    match run_callback("validate callback", &path.join(VALIDATE_CALLBACK)) {
        // We continue the transition in case the validation callback executes fine.
        Ok(_) => Ok(Transition::Continue),

        // In the case of the validation callback exits with error, we cancel
        // the transition so we can do a rollback of the update.
        Err(Error::Process(_)) => Ok(Transition::Cancel),

        // FIXME: We likely need to return Transition::Cancel here but we need
        // to check what are the possible error cases and verify if we cannot
        // handle some of them more gracefully.
        Err(e) => Err(e),
    }
}

pub(crate) fn rollback_callback(path: &Path) -> Result<()> {
    run_callback("rollback callback", &path.join(ROLLBACK_CALLBACK))
}

pub(crate) fn error_callback(path: &Path) -> Result<()> {
    run_callback("error callback", &path.join(ERROR_CALLBACK))
}

fn run_callback(name: &str, path: &Path) -> Result<()> {
    let callback = path.join(path);
    if !callback.exists() {
        return Ok(());
    }

    match easy_process::run(&callback.to_string_lossy()) {
        Ok(output) => {
            trace!("{} has exit with success", name);
            for err in output.stderr.lines() {
                error!("{} (stderr): {}", path.display(), err);
            }

            Ok(())
        }
        Err(easy_process::Error::Failure(status, output)) => {
            error!("{} has failed with status: {:?}", name, status);
            for err in output.stderr.lines() {
                error!("{} (stderr): {}", path.display(), err);
            }
            Err(easy_process::Error::Failure(status, output).into())
        }
        Err(e) => {
            error!("{} has failed with an invalid error: {:?}", name, e);
            Err(e.into())
        }
    }
}
