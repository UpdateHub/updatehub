// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{definitions, ObjectType};
use serde::Deserialize;

#[derive(Deserialize, PartialEq, Debug)]
#[serde(rename_all = "kebab-case")]
pub(crate) struct Ubifs {
    filename: String,
    size: u64,
    sha256sum: String,
    #[serde(flatten)]
    target: definitions::TargetType,

    compressed: Option<bool>,
    required_uncompressed_size: Option<u64>,
}

impl_object_type!(Ubifs);

#[test]
fn deserialize() {
    use pretty_assertions::assert_eq;
    use serde_json::json;

    assert_eq!(
        Ubifs {
            filename: "ubifs".to_string(),
            size: 1024,
            sha256sum: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
                .to_string(),
            target: definitions::TargetType::UBIVolume("home".to_string()),

            compressed: Some(true),
            required_uncompressed_size: Some(2048),
        },
        serde_json::from_value::<Ubifs>(json!({
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
