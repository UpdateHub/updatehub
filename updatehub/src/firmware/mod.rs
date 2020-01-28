// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use self::hook::{run_hook, run_hooks_from_dir};

use derive_more::{Display, From};
use serde::Serialize;
use std::path::Path;

mod hook;
mod metadata_value;

pub mod installation_set;

#[cfg(test)]
pub mod tests;

const PRODUCT_UID_HOOK: &str = "product-uid";
const VERSION_HOOK: &str = "version";
const HARDWARE_HOOK: &str = "hardware";
const DEVICE_IDENTITY_DIR: &str = "device-identity.d";
const DEVICE_ATTRIBUTES_DIR: &str = "device-attributes.d";

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

/// Metadata stores the firmware metadata information. It is
/// organized in multiple fields.
///
/// The Metadata is created loading its information from the running
/// firmware. It uses the `load` method for that.
#[derive(Debug, Serialize, PartialEq, Clone)]
#[serde(rename_all = "kebab-case")]
pub struct Metadata {
    /// Product UID which identifies the firmware on the management system
    pub product_uid: String,

    /// Version of firmware
    pub version: String,

    /// Hardware where the firmware is running
    pub hardware: String,

    /// Device Identity
    pub device_identity: metadata_value::MetadataValue,

    /// Device Attributes
    pub device_attributes: metadata_value::MetadataValue,
}

impl Metadata {
    pub fn from_path(path: &Path) -> Result<Self> {
        let product_uid_hook = path.join(PRODUCT_UID_HOOK);
        let version_hook = path.join(VERSION_HOOK);
        let hardware_hook = path.join(HARDWARE_HOOK);
        let device_identity_dir = path.join(DEVICE_IDENTITY_DIR);
        let device_attributes_dir = path.join(DEVICE_ATTRIBUTES_DIR);

        let metadata = Self {
            product_uid: run_hook(&product_uid_hook)?,
            version: run_hook(&version_hook)?,
            hardware: run_hook(&hardware_hook)?,
            device_identity: run_hooks_from_dir(&device_identity_dir)?,
            device_attributes: run_hooks_from_dir(&device_attributes_dir).unwrap_or_default(),
        };

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
