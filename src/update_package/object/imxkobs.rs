// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{definitions, ObjectType};
use serde::Deserialize;

#[derive(Deserialize, PartialEq, Debug)]
pub(crate) struct Imxkobs {
    filename: String,
    size: u64,
    sha256sum: String,

    install_if_different: Option<definitions::InstallIfDifferent>,
    #[serde(rename = "1k_padding")]
    #[serde(default)]
    padding_1k: bool,
    #[serde(default)]
    search_exponent: usize,
    #[serde(default)]
    chip_0_device_path: String,
    #[serde(default)]
    chip_1_device_path: String,
}

impl_object_type!(Imxkobs);

#[test]
fn deserialize() {
    use pretty_assertions::assert_eq;
    use serde_json::json;

    assert_eq!(
        Imxkobs {
            filename: "imxkobs-filename".to_string(),
            size: 1024,
            sha256sum: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
                .to_string(),

            install_if_different: None,
            padding_1k: true,
            search_exponent: 2,
            chip_0_device_path: "/dev/sda1".to_string(),
            chip_1_device_path: "/dev/sda2".to_string(),
        },
        serde_json::from_value::<Imxkobs>(json!({
            "filename": "imxkobs-filename",
            "sha256sum": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
            "size": 1024,
            "1k_padding": true,
            "search_exponent": 2,
            "chip_0_device_path": "/dev/sda1",
            "chip_1_device_path": "/dev/sda2",
        }))
        .unwrap()
    );
}
