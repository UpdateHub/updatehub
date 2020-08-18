// Copyright (C) 2021 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::definitions::TargetType;
use serde::Deserialize;

#[derive(Deserialize, PartialEq, Debug)]
#[serde(rename_all = "kebab-case")]
pub struct Bita {
    pub filename: String,
    pub sha256sum: String,
    #[serde(flatten)]
    pub target: TargetType,
    pub size: u64,
}

#[test]
fn deserialize() {
    use pretty_assertions::assert_eq;
    use serde_json::json;

    assert_eq!(
        Bita {
            filename: "etc/passwd".to_string(),
            sha256sum: "cfe2be1c64b0387500853de0f48303e3de7b1c6f1508dc719eeafa0d41c36722"
                .to_string(),
            target: TargetType::Device(std::path::PathBuf::from("/dev/sda1")),
            size: 1024,
        },
        serde_json::from_value::<Bita>(json!({
            "filename": "etc/passwd",
            "sha256sum": "cfe2be1c64b0387500853de0f48303e3de7b1c6f1508dc719eeafa0d41c36722",
            "target-type": "device",
            "target": "/dev/sda1",
            "size": 1024,
        }))
        .unwrap()
    );
}
