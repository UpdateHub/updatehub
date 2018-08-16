// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
//

use Result;

use hook::run_hook;
use std::path::Path;

mod metadata_value;
use self::metadata_value::MetadataValue;

mod hook;
use self::hook::run_hooks_from_dir;

#[cfg(test)]
pub mod tests;

const PRODUCT_UID_HOOK: &str = "product-uid";
const VERSION_HOOK: &str = "version";
const HARDWARE_HOOK: &str = "hardware";
const DEVICE_IDENTITY_DIR: &str = "device-identity.d";
const DEVICE_ATTRIBUTES_DIR: &str = "device-attributes.d";

#[derive(Fail, Debug)]
pub enum FirmwareError {
    #[fail(display = "Invalid product UID")]
    InvalidProductUid,
    #[fail(display = "Product UID is missing")]
    MissingProductUid,
    #[fail(display = "Device Identity is missing")]
    MissingDeviceIdentity,
}

/// Metadata stores the firmware metadata information. It is
/// organized in multiple fields.
///
/// The Metadata is created loading its information from the running
/// firmware. It uses the `load` method for that.
#[derive(Debug, Serialize, PartialEq)]
pub struct Metadata {
    /// Product UID which identifies the firmware on the management system
    pub product_uid: String,

    /// Version of firmware
    pub version: String,

    /// Hardware where the firmware is running
    pub hardware: String,

    /// Device Identity
    pub device_identity: MetadataValue,

    /// Device Attributes
    pub device_attributes: MetadataValue,
}

impl Metadata {
    pub fn new(path: &Path) -> Result<Metadata> {
        let product_uid_hook = path.join(PRODUCT_UID_HOOK);
        let version_hook = path.join(VERSION_HOOK);
        let hardware_hook = path.join(HARDWARE_HOOK);
        let device_identity_dir = path.join(DEVICE_IDENTITY_DIR);
        let device_attributes_dir = path.join(DEVICE_ATTRIBUTES_DIR);

        let metadata = Metadata {
            product_uid: run_hook(&product_uid_hook)?,
            version: run_hook(&version_hook)?,
            hardware: run_hook(&hardware_hook)?,
            device_identity: run_hooks_from_dir(&device_identity_dir)?,
            device_attributes: run_hooks_from_dir(&device_attributes_dir)?,
        };

        if metadata.product_uid.is_empty() {
            return Err(FirmwareError::MissingProductUid.into());
        }

        if metadata.product_uid.len() != 64 {
            return Err(FirmwareError::InvalidProductUid.into());
        }

        if metadata.device_identity.is_empty() {
            return Err(FirmwareError::MissingDeviceIdentity.into());
        }

        Ok(metadata)
    }
}
