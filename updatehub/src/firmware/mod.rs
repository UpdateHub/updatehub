// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use self::hook::{run_hook, run_hooks_from_dir};
use derive_more::{Deref, DerefMut, Display, From};
pub use sdk::api::info::firmware as api;
use slog_scope::error;
use std::path::Path;

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

pub type Result<T> = std::result::Result<T, Error>;

#[derive(Debug, Display, From)]
pub enum Error {
    #[display(fmt = "Invalid product UID")]
    InvalidProductUid,
    #[display(fmt = "Product UID is missing")]
    MissingProductUid,
    #[display(fmt = "Device Identity is missing")]
    MissingDeviceIdentity,

    #[display(fmt = "{} is a invalid value. The only know ones are 0 or 1", _0)]
    #[from(ignore)]
    InvalidInstallSet(u8),
    #[display(fmt = "ParseInt: {}", _0)]
    ParseInt(std::num::ParseIntError),
    #[display(fmt = "Walkdir error: {}", _0)]
    Walkdir(walkdir::Error),
    #[display(fmt = "Io error: {}", _0)]
    Io(std::io::Error),
    #[display(fmt = "Process error: {}", _0)]
    Process(easy_process::Error),
}

#[derive(Debug, PartialEq)]
pub(crate) enum Transition {
    Continue,
    Cancel,
}

#[derive(Clone, Debug, Deref, DerefMut, From, PartialEq)]
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
    use std::io;

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
