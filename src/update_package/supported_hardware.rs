// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
//

use failure::Error;
use serde_helpers::de::supported_hardware_any as any;
use update_package::UpdatePackageError;

#[derive(Debug, PartialEq, Deserialize)]
#[serde(untagged)]
pub enum SupportedHardware {
    #[serde(deserialize_with = "any")]
    Any,
    HardwareList(Vec<String>),
}

impl SupportedHardware {
    pub fn compatible_with(&self, hardware: &str) -> Result<(), Error> {
        let hardware = hardware.to_string();
        let compatible = match self {
            SupportedHardware::Any => true,
            SupportedHardware::HardwareList(l) => l.contains(&hardware),
        };

        if !compatible {
            return Err(UpdatePackageError::IncompatibleHardware(hardware).into());
        }

        Ok(())
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
    fn no_hardware() {
        assert!(serde_json::from_str::<Test>("").is_err());
    }

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
            Test(SupportedHardware::HardwareList(vec!["hw".to_string()])),
            serde_json::from_str::<Test>(&json!(["hw"]).to_string()).unwrap()
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
