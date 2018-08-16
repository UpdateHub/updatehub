// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
//

use Result;

use crypto_hash::{hex_digest, Algorithm};
use serde_json;

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

#[derive(Fail, Debug)]
pub enum UpdatePackageError {
    #[fail(display = "Incompatible with hardware: {}", _0)]
    IncompatibleHardware(String),
}

impl UpdatePackage {
    pub fn parse(content: &str) -> Result<Self> {
        let mut update_package = serde_json::from_str::<UpdatePackage>(content)?;
        update_package.raw = content.into();

        Ok(update_package)
    }

    pub fn package_uid(&self) -> String {
        hex_digest(Algorithm::SHA256, self.raw.as_bytes())
    }

    pub fn compatible_with(&self, firmware: &Metadata) -> Result<()> {
        self.supported_hardware.compatible_with(&firmware.hardware)
    }

    pub fn objects(&self) -> &Vec<Object> {
        &self.objects
    }

    pub fn filter_objects(&self, settings: &Settings, filter: &ObjectStatus) -> Vec<&Object> {
        self.objects
            .iter()
            .filter(|o| {
                o.status(&settings.update.download_dir)
                    .map_err(|e| {
                        error!("Fail accessing the object: {} (err: {})", o.sha256sum(), e)
                    }).unwrap_or(ObjectStatus::Missing)
                    .eq(filter)
            }).collect()
    }
}
