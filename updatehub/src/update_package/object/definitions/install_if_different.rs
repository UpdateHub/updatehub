// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use serde::Deserialize;

// This property handles when an object should be installed on target
#[derive(PartialEq, Debug, Deserialize)]
#[serde(untagged)]
pub enum InstallIfDifferent {
    CheckSum(CheckSum),
    Known(KnownPattern),
    Custom(CustomPattern),
}

#[derive(PartialEq, Debug, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum CheckSum {
    Sha256Sum,
}

#[derive(PartialEq, Debug, Deserialize)]
pub struct KnownPattern {
    version: String,
    pattern: KnowPatterns,
}

#[derive(PartialEq, Debug, Deserialize)]
#[serde(rename_all = "kebab-case")]
pub enum KnowPatterns {
    LinuxKernel,
    UBoot,
}

#[derive(PartialEq, Debug, Deserialize)]
pub struct CustomPattern {
    version: String,
    pattern: Pattern,
}

#[derive(PartialEq, Debug, Deserialize)]
#[serde(rename_all = "kebab-case")]
pub struct Pattern {
    regexp: String,
    seek: u64,
    buffer_size: u64,
}

#[cfg(test)]
mod test {
    use super::*;
    use pretty_assertions::assert_eq;
    use serde_json::json;

    #[test]
    fn deserialize() {
        assert_eq!(
            InstallIfDifferent::CheckSum(CheckSum::Sha256Sum),
            serde_json::from_value::<InstallIfDifferent>(json!("sha256sum")).unwrap()
        );
        assert_eq!(
            InstallIfDifferent::Custom(CustomPattern {
                version: "2.0".to_string(),
                pattern: Pattern {
                    regexp: "[0-9.]+".to_string(),
                    seek: 1024,
                    buffer_size: 2048,
                }
            }),
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
            InstallIfDifferent::Known(KnownPattern {
                version: "4.7.4-1-ARCH".to_string(),
                pattern: KnowPatterns::LinuxKernel,
            }),
            serde_json::from_value::<InstallIfDifferent>(json!({
                "version": "4.7.4-1-ARCH",
                "pattern": "linux-kernel"
            }))
            .unwrap()
        )
    }
}
