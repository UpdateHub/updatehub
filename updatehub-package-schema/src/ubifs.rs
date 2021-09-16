// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::definitions::TargetType;
use serde::Deserialize;

#[derive(Clone, Deserialize, PartialEq, Debug)]
#[serde(rename_all = "kebab-case")]
pub struct Ubifs {
    pub filename: String,
    pub size: u64,
    pub sha256sum: String,
    #[serde(flatten)]
    pub target: TargetType,

    #[serde(default)]
    pub compressed: bool,
    #[serde(default)]
    pub required_uncompressed_size: u64,
}

#[test]
fn deserialize() {
    use pretty_assertions::assert_eq;
    use serde_json::json;

    assert_eq!(
        super::Object::Ubifs(Box::new(Ubifs {
            filename: "ubifs".to_string(),
            size: 1024,
            sha256sum: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
                .to_string(),
            target: TargetType::UBIVolume("home".to_string()),

            compressed: true,
            required_uncompressed_size: 2048,
        })),
        serde_json::from_value::<super::Object>(json!({
            "mode": "ubifs",
            "filename": "ubifs",
            "sha256sum": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
            "size": 1024,
            "target-type": "ubivolume",
            "target": "home",
            "compressed": true,
            "required-uncompressed-size": 2048
        }))
        .unwrap()
    );
}
