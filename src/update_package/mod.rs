// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
//

use crypto_hash::{hex_digest, Algorithm};
use serde_json::{self, Error};

use firmware::Metadata;
use settings::Settings;

mod supported_hardware;
use self::supported_hardware::SupportedHardware;

#[macro_use]
mod macros;

mod object;
use self::object::Object;
pub use self::object::ObjectStatus;

#[cfg(test)]
pub mod tests;

// CHECK: https://play.rust-lang.org/?gist=b7bc6ad2c073692f96007928aac75768&version=stable
// It does show how to match the different object types

#[derive(Debug, PartialEq, Deserialize)]
#[serde(rename_all = "kebab-case")]
pub struct UpdatePackage {
    product_uid: String,
    version: String,

    #[serde(default)]
    supported_hardware: SupportedHardware,

    objects: Vec<Object>,

    #[serde(skip_deserializing)]
    raw: String,
}

impl UpdatePackage {
    pub fn parse(content: &str) -> Result<Self, Error> {
        let mut update_package = serde_json::from_str::<UpdatePackage>(content)?;
        update_package.raw = content.into();

        Ok(update_package)
    }

    pub fn package_uid(&self) -> Option<String> {
        Some(hex_digest(Algorithm::SHA256, self.raw.as_bytes()))
    }

    pub fn compatible_with(&self, firmware: &Metadata) -> bool {
        match self.supported_hardware {
            SupportedHardware::Any => true,
            SupportedHardware::Hardware(ref s) => s == &firmware.hardware,
            SupportedHardware::HardwareList(ref l) => l.contains(&firmware.hardware),
        }
    }

    pub fn objects(&self) -> &Vec<Object> {
        &self.objects
    }

    pub fn filter_objects(&self, settings: &Settings, filter: ObjectStatus) -> Vec<&Object> {
        self.objects
            .iter()
            .filter(|o| {
                let status = o.status(&settings.update.download_dir)
                    .map_err(|err| {
                        error!(
                            "Fail accessing the object: {} (err: {})",
                            o.sha256sum(),
                            err
                        )
                    })
                    .unwrap_or(ObjectStatus::Missing);

                status == filter
            })
            .collect()
    }
}
