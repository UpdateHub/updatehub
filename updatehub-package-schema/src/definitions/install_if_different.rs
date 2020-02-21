// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use serde::Deserialize;

/// Handles when an object should be installed on target.
#[derive(PartialEq, Debug, Deserialize)]
#[serde(untagged)]
pub enum InstallIfDifferent {
    #[serde(deserialize_with = "deserialize_checksum")]
    /// Use checksum to check.
    CheckSum,
    /// Use a predefined (known) pattern to check.
    KnownPattern { version: String, pattern: KnownPatternKind },
    /// Use a custom pattern to check.
    CustomPattern { version: String, pattern: Pattern },
}

/// Known patterns to be used with
/// [`InstallIfDifferent`](InstallIfDifferent::KnownPattern)
#[derive(PartialEq, Debug, Deserialize)]
#[serde(rename_all = "kebab-case")]
pub enum KnownPatternKind {
    LinuxKernel,
    UBoot,
}

/// Custom pattern to use with
/// [`InstallIfDifferent`](InstallIfDifferent::CustomPattern)
#[derive(PartialEq, Debug, Deserialize)]
#[serde(rename_all = "kebab-case")]
pub struct Pattern {
    pub regexp: String,
    pub seek: u64,
    pub buffer_size: u64,
}

fn deserialize_checksum<'de, D, E>(deserializer: D) -> Result<(), E>
where
    D: serde::Deserializer<'de>,
    E: serde::de::Error + From<D::Error>,
{
    match String::deserialize(deserializer)?.to_lowercase().as_str() {
        "sha256sum" => Ok(()),
        s => Err(E::custom(format!("Not a vliad CheckSum format: {}", s))),
    }
}

#[cfg(test)]
mod test {
    use super::*;
    use pretty_assertions::assert_eq;
    use serde_json::json;

    #[test]
    fn deserialize() {
        assert_eq!(
            InstallIfDifferent::CheckSum,
            serde_json::from_value::<InstallIfDifferent>(json!("sha256sum")).unwrap()
        );
        assert_eq!(
            InstallIfDifferent::CustomPattern {
                version: "2.0".to_string(),
                pattern: Pattern { regexp: "[0-9.]+".to_string(), seek: 1024, buffer_size: 2048 }
            },
            serde_json::from_value::<InstallIfDifferent>(json!({
                "version": "2.0",
                "pattern": {
                    "regexp": "[0-9.]+",
                    "seek": 1024,
                    "buffer-size": 2048
                }
            }))
            .unwrap()
        );
        assert_eq!(
            InstallIfDifferent::KnownPattern {
                version: "4.7.4-1-ARCH".to_string(),
                pattern: KnownPatternKind::LinuxKernel,
            },
            serde_json::from_value::<InstallIfDifferent>(json!({
                "version": "4.7.4-1-ARCH",
                "pattern": "linux-kernel"
            }))
            .unwrap()
        )
    }
}
