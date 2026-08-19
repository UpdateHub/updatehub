// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use serde::Serialize;
use std::{collections::BTreeMap, fmt::Write, fs, path::Path};

#[derive(Debug)]
pub enum ProbeResponse {
    NoUpdate,
    Update(UpdatePackage, Option<Signature>),
    ExtraPoll(i64),
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct UpdatePackage {
    pub inner: pkg_schema::UpdatePackage,
    pub raw: Vec<u8>,
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Signature(Vec<u8>);

#[derive(Serialize)]
#[serde(rename_all = "kebab-case")]
pub struct FirmwareMetadata<'a> {
    pub product_uid: &'a str,
    pub version: &'a str,
    pub hardware: &'a str,
    pub device_identity: MetadataValue<'a>,
    pub device_attributes: MetadataValue<'a>,
}

pub struct MetadataValue<'a>(pub &'a BTreeMap<String, Vec<String>>);

impl serde::ser::Serialize for MetadataValue<'_> {
    fn serialize<S>(&self, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: serde::ser::Serializer,
    {
        use serde::ser::SerializeMap;

        let mut map = serializer.serialize_map(Some(self.0.len()))?;
        for (k, v) in self.0 {
            if v.len() == 1 {
                map.serialize_entry(k, &v[0])?;
            } else {
                map.serialize_entry(k, v)?;
            }
        }
        map.end()
    }
}

impl UpdatePackage {
    /// Parses the raw metadata of an update package.
    ///
    /// # Errors
    ///
    /// Returns an error when `content` is not the JSON document the agent
    /// expects.
    pub fn parse(content: &[u8]) -> crate::Result<Self> {
        let update_package = serde_json::from_slice(content)?;
        Ok(UpdatePackage { inner: update_package, raw: content.to_vec() })
    }

    /// Returns the SHA-256 sum of the raw metadata, which identifies the
    /// package.
    #[must_use]
    pub fn package_uid(&self) -> String {
        openssl::sha::sha256(&self.raw).iter().fold(String::new(), |mut output, c| {
            let _ = write!(output, "{c:02x}");

            output
        })
    }

    /// Returns the version the package declares.
    #[must_use]
    pub fn version(&self) -> &str {
        &self.inner.version
    }
}

impl Signature {
    /// Decodes a signature from its base64 form.
    ///
    /// # Errors
    ///
    /// Returns an error when `bytes` is not valid base64.
    pub fn from_base64_str(bytes: &str) -> crate::Result<Self> {
        Ok(Signature(openssl::base64::decode_block(bytes)?))
    }

    /// Checks the signature of `package` against the public key stored at
    /// `key`.
    ///
    /// # Errors
    ///
    /// Returns an error when the key does not load, when the check itself
    /// fails, or when the signature does not match the package.
    pub fn validate(&self, key: &Path, package: &UpdatePackage) -> crate::Result<()> {
        use openssl::{hash::MessageDigest, pkey::PKey, rsa::Rsa, sign::Verifier};
        let key = PKey::from_rsa(Rsa::public_key_from_pem(&fs::read(key)?)?)?;
        if Verifier::new(MessageDigest::sha256(), &key)?.verify_oneshot(&self.0, &package.raw)? {
            return Ok(());
        }
        Err(crate::Error::InvalidSignature)
    }
}
