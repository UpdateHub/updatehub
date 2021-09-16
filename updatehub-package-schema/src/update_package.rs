// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use serde::Deserialize;

#[derive(Clone, Debug, PartialEq, Deserialize)]
pub struct UpdatePackage {
    #[serde(rename = "product")]
    pub product_uid: String,
    pub version: String,
    #[serde(default, rename = "supported-hardware")]
    pub supported_hardware: SupportedHardware,
    pub objects: (Vec<crate::Object>, Vec<crate::Object>),
}

#[derive(Clone, Debug, PartialEq, Deserialize)]
#[serde(untagged)]
pub enum SupportedHardware {
    #[serde(deserialize_with = "any")]
    Any,
    HardwareList(Vec<String>),
}

impl Default for SupportedHardware {
    fn default() -> Self {
        SupportedHardware::Any
    }
}

fn any<'de, D: serde::de::Deserializer<'de>>(deserializer: D) -> Result<(), D::Error> {
    if String::deserialize(deserializer)? == "any" {
        Ok(())
    } else {
        Err(serde::de::Error::custom("expected \"any\""))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use pretty_assertions::assert_eq;
    use serde_json::json;

    #[test]
    fn no_hardware() {
        assert!(serde_json::from_str::<SupportedHardware>("").is_err());
    }

    #[test]
    fn any_hardware() {
        assert_eq!(
            SupportedHardware::Any,
            serde_json::from_str::<SupportedHardware>(&json!("any").to_string()).unwrap()
        );
    }

    #[test]
    fn one_hardware() {
        assert_eq!(
            SupportedHardware::HardwareList(vec!["hw".to_string()]),
            serde_json::from_str::<SupportedHardware>(&json!(["hw"]).to_string()).unwrap()
        );
    }

    #[test]
    fn many_hardware() {
        assert_eq!(
            SupportedHardware::HardwareList(vec!["hw-1".into(), "hw-2".into()]),
            serde_json::from_str::<SupportedHardware>(&json!(["hw-1", "hw-2"]).to_string())
                .unwrap()
        );
    }
}
