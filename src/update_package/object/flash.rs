// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{definitions, ObjectType};
use serde::Deserialize;

#[derive(Deserialize, PartialEq, Debug)]
#[serde(rename_all = "kebab-case")]
pub(crate) struct Flash {
    filename: String,
    size: u64,
    sha256sum: String,
    #[serde(flatten)]
    target: definitions::TargetType,

    install_if_different: Option<definitions::InstallIfDifferent>,
}

impl_object_type!(Flash);

#[test]
fn deserialize() {
    use pretty_assertions::assert_eq;
    use serde_json::json;

    assert_eq!(
        Flash {
            filename: "etc/passwd".to_string(),
            size: 1024,
            sha256sum: "cfe2be1c64b0387500853de0f48303e3de7b1c6f1508dc719eeafa0d41c36722"
                .to_string(),
            target: definitions::TargetType::Device("/dev/sda".to_string()),

            install_if_different: None,
        },
        serde_json::from_value::<Flash>(json!({
            "filename": "etc/passwd",
            "size": 1024,
            "sha256sum": "cfe2be1c64b0387500853de0f48303e3de7b1c6f1508dc719eeafa0d41c36722",
            "target-type": "device",
            "target": "/dev/sda",
        }))
        .unwrap()
    );
}
