// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use serde::Deserialize;
use std::path::PathBuf;

/// The type the device that will receive the update.
#[derive(Clone, Debug, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase", tag = "target-type", content = "target")]
pub enum TargetType {
    Device(PathBuf),
    UBIVolume(String),
    MTDName(String),
    /// The base device of an eMMC (e.g. `/dev/mmcblk1`) whose hardware boot
    /// partitions hold one firmware per installation set: the agent writes
    /// the boot partition of the set being installed (`boot0` for set 0,
    /// `boot1` for set 1). Only valid for raw objects.
    #[serde(rename = "boot-emmc")]
    BootEmmc(PathBuf),
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
