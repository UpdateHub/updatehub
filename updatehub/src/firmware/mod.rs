// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use self::hook::{run_hook, run_hooks_from_dir};
use derive_more::{Deref, DerefMut};
pub use sdk::api::info::firmware as api;
use slog_scope::error;
use std::{io, path::Path};
use thiserror::Error;

mod hook;

pub mod installation_set;

#[cfg(test)]
pub mod tests;

const PRODUCT_UID_HOOK: &str = "product-uid";
const VERSION_HOOK: &str = "version";
const HARDWARE_HOOK: &str = "hardware";
const DEVICE_IDENTITY_DIR: &str = "device-identity.d";
const DEVICE_ATTRIBUTES_DIR: &str = "device-attributes.d";
const STATE_CHANGE_CALLBACK: &str = "state-change-callback";
const VALIDATE_CALLBACK: &str = "validate-callback";
const ROLLBACK_CALLBACK: &str = "rollback-callback";
const ERROR_CALLBACK: &str = "error-callback";

pub type Result<T> = std::result::Result<T, Error>;

#[derive(Debug, Error)]
pub enum Error {
    #[error("Invalid product UID")]
    InvalidProductUid,

    #[error("Product UID is missing")]
    MissingProductUid,

    #[error("Device Identity is missing")]
    MissingDeviceIdentity,

    #[error("{0} is a invalid value. The only know ones are 0 or 1")]
    InvalidInstallSet(u8),

    #[error("ParseInt: {0}")]
    ParseInt(#[from] std::num::ParseIntError),

    #[error("Walkdir error: {0}")]
    Walkdir(#[from] walkdir::Error),

    #[error("Io error: {0}")]
    Io(#[from] std::io::Error),

    #[error("Process error: {0}")]
    Process(#[from] easy_process::Error),
}

#[derive(Debug, PartialEq)]
pub(crate) enum Transition {
    Continue,
    Cancel,
}

#[derive(Clone, Debug, Deref, DerefMut, PartialEq)]
pub(crate) struct Metadata(pub(crate) api::Metadata);

impl Metadata {
    pub fn from_path(path: &Path) -> Result<Self> {
        let product_uid_hook = path.join(PRODUCT_UID_HOOK);
        let version_hook = path.join(VERSION_HOOK);
        let hardware_hook = path.join(HARDWARE_HOOK);
        let device_identity_dir = path.join(DEVICE_IDENTITY_DIR);
        let device_attributes_dir = path.join(DEVICE_ATTRIBUTES_DIR);

        let metadata = Metadata(api::Metadata {
            product_uid: run_hook(&product_uid_hook)?,
            version: run_hook(&version_hook)?,
            hardware: run_hook(&hardware_hook)?,
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
                "Invalid format found while running 'state-change-callback' \
                 hook for state '{}'",
                &state
            ),
        )
        .into()),
    }
}

pub(crate) fn validate_callback(path: &Path) -> Result<Transition> {
    let callback = path.join(VALIDATE_CALLBACK);
    if !callback.exists() {
        return Ok(Transition::Continue);
    }

    match easy_process::run(&callback.to_string_lossy()) {
        Ok(output) => {
            for err in output.stderr.lines() {
                error!("{} (stderr): {}", path.display(), err);
            }
            Ok(Transition::Continue)
        }
        Err(easy_process::Error::Failure(status, output)) => {
            error!("Validation callback has failed with status: {:?}", status);
            for err in output.stderr.lines() {
                error!("{} (stderr): {}", path.display(), err);
            }
            Ok(Transition::Cancel)
        }
        Err(e) => Err(e.into()),
    }
}

pub(crate) fn rollback_callback(path: &Path) -> Result<()> {
    let rollback = path.join(ROLLBACK_CALLBACK);
    if !rollback.exists() {
        return Ok(());
    }

    let output = easy_process::run(&rollback.to_string_lossy())?;
    for err in output.stderr.lines() {
        error!("{} (stderr): {}", path.display(), err);
    }

    Ok(())
}

pub(crate) fn error_callback(path: &Path) -> Result<()> {
    let error = path.join(ERROR_CALLBACK);
    if !error.exists() {
        return Ok(());
    }

    let output = easy_process::run(&error.to_string_lossy())?;
    for err in output.stderr.lines() {
        error!("{} (stderr): {}", path.display(), err);
    }

    Ok(())
}
