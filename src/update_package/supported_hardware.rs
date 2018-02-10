// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
// 

use serde::{self, Deserialize, Deserializer};

#[derive(Debug, PartialEq, Deserialize)]
#[serde(untagged)]
pub enum SupportedHardware {
    #[serde(deserialize_with = "any")]
    Any,
    Hardware(String),
    HardwareList(Vec<String>),
}

fn any<'de, D>(deserializer: D) -> Result<(), D::Error>
where
    D: Deserializer<'de>,
{
    if String::deserialize(deserializer)? == "any" {
        Ok(())
    } else {
        Err(serde::de::Error::custom("expected \"any\""))
    }
}

impl Default for SupportedHardware {
    fn default() -> Self {
        SupportedHardware::Any
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json;

    #[derive(Debug, Deserialize, PartialEq)]
    struct Test(SupportedHardware);

    #[test]
    fn any_hardware() {
        assert_eq!(
            Test(SupportedHardware::Any),
            serde_json::from_str::<Test>(&json!("any").to_string()).unwrap()
        );
    }

    #[test]
    fn one_hardware() {
        assert_eq!(
            Test(SupportedHardware::Hardware("hw".into())),
            serde_json::from_str::<Test>(&json!("hw").to_string()).unwrap()
        );
    }

    #[test]
    fn many_hardware() {
        assert_eq!(
            Test(SupportedHardware::HardwareList(vec![
                "hw-1".into(),
                "hw-2".into(),
            ])),
            serde_json::from_str::<Test>(&json!(["hw-1", "hw-2"]).to_string()).unwrap()
        );
    }
}
