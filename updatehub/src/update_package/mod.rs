// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::{
    firmware::{installation_set::Set, Metadata},
    object::{self, Info},
    settings::Settings,
};
use sdk::api::info::runtime_settings::InstallationSet;
use walkdir::WalkDir;

use crypto_hash::{hex_digest, Algorithm};

use pkg_schema::Object;
use serde::Deserialize;
use slog_scope::error;
use thiserror::Error;

use std::{fs, io, path::Path};

mod supported_hardware;
use self::supported_hardware::SupportedHardware;

#[cfg(test)]
pub(crate) mod tests;

// CHECK: https://play.rust-lang.org/?gist=b7bc6ad2c073692f96007928aac75768&version=stable
// It does show how to match the different object types

#[derive(Debug, PartialEq)]
pub(crate) struct UpdatePackage {
    product_uid: String,
    version: String,
    supported_hardware: SupportedHardware,
    objects: (Vec<Object>, Vec<Object>),
    raw: Vec<u8>,
}

#[derive(Debug)]
pub(crate) struct Signature(pub(crate) Vec<u8>);

pub type Result<T> = std::result::Result<T, Error>;

#[derive(Debug, Error)]
pub enum Error {
    #[error("Json parsing error: {0}")]
    JsonParsing(#[from] serde_json::Error),
    #[error("IO error: {0}")]
    Io(#[from] std::io::Error),
    #[error("OpenSSL error: {0}")]
    OpenSsl(#[from] openssl::error::ErrorStack),

    #[error("Incompatible with hardware: {0}")]
    IncompatibleHardware(String),
    #[error("Package's signature validation has failed")]
    InvalidSignature,
}

impl UpdatePackage {
    pub(crate) fn parse(content: &[u8]) -> Result<Self> {
        #[derive(Deserialize)]
        #[serde(rename_all = "kebab-case")]
        struct UpdatePackageAux {
            #[serde(rename = "product")]
            product_uid: String,
            version: String,
            #[serde(default)]
            supported_hardware: SupportedHardware,
            objects: (Vec<Object>, Vec<Object>),
        }

        let update_package = serde_json::from_slice::<UpdatePackageAux>(content)?;
        Ok(UpdatePackage {
            product_uid: update_package.product_uid,
            version: update_package.version,
            supported_hardware: update_package.supported_hardware,
            objects: update_package.objects,
            raw: content.to_vec(),
        })
    }

    pub(crate) fn package_uid(&self) -> String {
        hex_digest(Algorithm::SHA256, &self.raw)
    }

    pub(crate) fn compatible_with(&self, firmware: &Metadata) -> Result<()> {
        self.supported_hardware.compatible_with(&firmware.hardware)
    }

    pub(crate) fn objects(&self, installation_set: Set) -> &Vec<Object> {
        match installation_set.0 {
            InstallationSet::A => &self.objects.0,
            InstallationSet::B => &self.objects.1,
        }
    }

    pub(crate) fn objects_mut(&mut self, installation_set: Set) -> &mut Vec<Object> {
        match installation_set.0 {
            InstallationSet::A => &mut self.objects.0,
            InstallationSet::B => &mut self.objects.1,
        }
    }

    pub(crate) fn filter_objects(
        &self,
        settings: &Settings,
        installation_set: Set,
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

    pub(crate) fn clear_unrelated_files(
        &self,
        dir: &Path,
        installation_set: Set,
        settings: &Settings,
    ) -> io::Result<()> {
        // Prune left over objects from previous installations
        for entry in WalkDir::new(dir)
            .follow_links(true)
            .min_depth(1)
            .into_iter()
            .filter_entry(|e| e.file_type().is_file())
            .filter_map(std::result::Result::ok)
            .filter(|e| {
                !self
                    .objects(installation_set)
                    .iter()
                    .map(object::Info::sha256sum)
                    .any(|x| x == e.file_name())
            })
        {
            fs::remove_file(entry.path())?;
        }

        // Cleanup metadata and signature for older local local installation
        for file in &[dir.join("metadata"), dir.join("signature")] {
            if file.exists() {
                fs::remove_file(file)?;
            }
        }

        // Prune corrupted files
        for object in
            self.filter_objects(&settings, installation_set, object::info::Status::Corrupted)
        {
            fs::remove_file(dir.join(object.sha256sum()))?;
        }

        Ok(())
    }
}

impl Signature {
    pub(crate) fn from_str(bytes: &str) -> Result<Self> {
        Ok(Signature(openssl::base64::decode_block(bytes)?.to_vec()))
    }

    pub(crate) fn validate(&self, key: &Path, package: &UpdatePackage) -> Result<()> {
        use openssl::{hash::MessageDigest, pkey::PKey, rsa::Rsa, sign::Verifier};
        let key = PKey::from_rsa(Rsa::public_key_from_pem(&fs::read(key)?)?)?;
        if Verifier::new(MessageDigest::sha256(), &key)?.verify_oneshot(&self.0, &package.raw)? {
            return Ok(());
        }
        Err(Error::InvalidSignature)
    }
}
