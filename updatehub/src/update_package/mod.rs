// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::{
    firmware::{installation_set::Set as InstallationSet, Metadata},
    object::{self, Info},
    settings::Settings,
};

use crypto_hash::{hex_digest, Algorithm};

use derive_more::{Display, From};
use pkg_schema::Object;
use serde::Deserialize;
use serde_json;
use slog_scope::error;

mod supported_hardware;
use self::supported_hardware::SupportedHardware;

#[cfg(test)]
pub(crate) mod tests;

// CHECK: https://play.rust-lang.org/?gist=b7bc6ad2c073692f96007928aac75768&version=stable
// It does show how to match the different object types

#[derive(Debug, PartialEq, Deserialize)]
#[serde(rename_all = "kebab-case")]
pub(crate) struct UpdatePackage {
    #[serde(rename = "product")]
    product_uid: String,
    version: String,

    #[serde(default)]
    supported_hardware: SupportedHardware,

    objects: (Vec<Object>, Vec<Object>),

    #[serde(skip_deserializing)]
    raw: String,
}

pub type Result<T> = std::result::Result<T, Error>;

#[derive(Debug, Display, From)]
pub enum Error {
    #[display(fmt = "Json parsing error: {}", _0)]
    JsonParsing(serde_json::Error),

    #[display(fmt = "Incompatible with hardware: {}", _0)]
    #[from(ignore)]
    IncompatibleHardware(String),
}

impl UpdatePackage {
    pub(crate) fn parse(content: &str) -> Result<Self> {
        let mut update_package = serde_json::from_str::<Self>(content)?;
        update_package.raw = content.into();

        Ok(update_package)
    }

    pub(crate) fn package_uid(&self) -> String {
        hex_digest(Algorithm::SHA256, self.raw.as_bytes())
    }

    pub(crate) fn compatible_with(&self, firmware: &Metadata) -> Result<()> {
        self.supported_hardware.compatible_with(&firmware.hardware)
    }

    pub(crate) fn objects(&self, installation_set: InstallationSet) -> &Vec<Object> {
        match installation_set {
            InstallationSet::A => &self.objects.0,
            InstallationSet::B => &self.objects.1,
        }
    }

    pub(crate) fn objects_mut(&mut self, installation_set: InstallationSet) -> &mut Vec<Object> {
        match installation_set {
            InstallationSet::A => &mut self.objects.0,
            InstallationSet::B => &mut self.objects.1,
        }
    }

    pub(crate) fn filter_objects(
        &self,
        settings: &Settings,
        installation_set: InstallationSet,
        filter: object::info::Status,
    ) -> Vec<&Object> {
        self.objects(installation_set)
            .iter()
            .filter(|o| {
                o.status(&settings.update.download_dir)
                    .map_err(|e| {
                        error!("Fail accessing the object: {} (err: {})", o.sha256sum(), e)
                    })
                    .unwrap_or(object::info::Status::Missing)
                    .eq(&filter)
            })
            .collect()
    }
}
