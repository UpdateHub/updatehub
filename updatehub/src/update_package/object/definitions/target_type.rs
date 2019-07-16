// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use failure::ensure;
use serde::Deserialize;
use std::path::PathBuf;

#[derive(PartialEq, Debug, Deserialize)]
#[serde(rename_all = "lowercase", tag = "target-type", content = "target")]
pub enum TargetType {
    Device(PathBuf),
    UBIVolume(String),
    MTDName(String),
}

impl TargetType {
    pub fn valid(&self) -> Result<&Self, failure::Error> {
        Ok(match self {
            TargetType::Device(p) => {
                ensure!(p.exists(), "Target device does not exists");
                ensure!(
                    !p.metadata()?.permissions().readonly(),
                    "User doesn't have write permission on target device"
                );
                &self
            }
            TargetType::UBIVolume(_) => unimplemented!("FIXME: Check if UBI Volume is valid"),
            TargetType::MTDName(_) => unimplemented!("FIXME: Check if MTD name is valid"),
        })
    }

    pub fn get_target(&self) -> Result<PathBuf, failure::Error> {
        match self {
            TargetType::Device(p) => Ok(p.clone()),
            TargetType::UBIVolume(_s) => unimplemented!("FIXME: Get device from UBI Volume name"),
            TargetType::MTDName(_s) => unimplemented!("FIXME: Get device from MTD name"),
        }
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
            TargetType::Device(PathBuf::from("/dev/sdb")),
            serde_json::from_value::<TargetType>(json!({
                "target-type": "device",
                "target": "/dev/sdb",
            }))
            .unwrap()
        );
        assert_eq!(
            TargetType::UBIVolume("system1".to_string()),
            serde_json::from_value::<TargetType>(json!({
                "target-type": "ubivolume",
                "target": "system1",
            }))
            .unwrap()
        );
    }
}
