// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::definitions::{InstallIfDifferent, TargetType};
use serde::Deserialize;

#[derive(Clone, Deserialize, PartialEq, Debug)]
#[serde(rename_all = "kebab-case")]
pub struct Flash {
    pub filename: String,
    pub size: u64,
    pub sha256sum: String,
    #[serde(flatten)]
    pub target: TargetType,

    pub install_if_different: Option<InstallIfDifferent>,
}

#[test]
fn deserialize() {
    use pretty_assertions::assert_eq;
    use serde_json::json;

    assert_eq!(
        super::Object::Flash(Box::new(Flash {
            filename: "etc/passwd".to_string(),
            size: 1024,
            sha256sum: "cfe2be1c64b0387500853de0f48303e3de7b1c6f1508dc719eeafa0d41c36722"
                .to_string(),
            target: TargetType::Device(std::path::PathBuf::from("/dev/sda")),

            install_if_different: None,
        })),
        serde_json::from_value::<super::Object>(json!({
            "mode": "flash",
            "filename": "etc/passwd",
            "size": 1024,
            "sha256sum": "cfe2be1c64b0387500853de0f48303e3de7b1c6f1508dc719eeafa0d41c36722",
            "target-type": "device",
            "target": "/dev/sda",
        }))
        .unwrap()
    );
}
