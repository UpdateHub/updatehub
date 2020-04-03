// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

mod supported_hardware;

use self::supported_hardware::SupportedHardwareExt;
use crate::{
    firmware::{installation_set::Set, Metadata},
    object::{self, Info},
    settings::Settings,
    utils,
};
use pkg_schema::Object;
use sdk::api::info::runtime_settings::InstallationSet;
use slog_scope::error;
use std::{fs, io, path::Path};
use thiserror::Error;
use walkdir::WalkDir;

#[cfg(test)]
pub(crate) mod tests;

#[derive(Debug, PartialEq)]
pub(crate) struct UpdatePackage {
    inner: pkg_schema::UpdatePackage,
    raw: Vec<u8>,
}

#[derive(Debug, PartialEq)]
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
        let update_package = serde_json::from_slice(content)?;
        Ok(UpdatePackage { inner: update_package, raw: content.to_vec() })
    }

    pub(crate) fn package_uid(&self) -> String {
        utils::sha256sum(&self.raw)
    }

    pub(crate) fn compatible_with(&self, firmware: &Metadata) -> Result<()> {
        self.inner.supported_hardware.compatible_with(&firmware.hardware)
    }

    pub(crate) fn objects(&self, installation_set: Set) -> &Vec<Object> {
        match installation_set.0 {
            InstallationSet::A => &self.inner.objects.0,
            InstallationSet::B => &self.inner.objects.1,
        }
    }

    pub(crate) fn objects_mut(&mut self, installation_set: Set) -> &mut Vec<Object> {
        match installation_set.0 {
            InstallationSet::A => &mut self.inner.objects.0,
            InstallationSet::B => &mut self.inner.objects.1,
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
