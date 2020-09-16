// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::definitions::InstallIfDifferent;
use serde::Deserialize;
use std::path::PathBuf;

#[derive(Deserialize, PartialEq, Debug)]
pub struct Imxkobs {
    pub filename: String,
    pub size: u64,
    pub sha256sum: String,

    pub install_if_different: Option<InstallIfDifferent>,
    #[serde(rename = "1k_padding")]
    #[serde(default)]
    pub padding_1k: bool,
    #[serde(default)]
    pub search_exponent: usize,
    #[serde(default)]
    pub chip_0_device_path: Option<PathBuf>,
    #[serde(default)]
    pub chip_1_device_path: Option<PathBuf>,
}

#[test]
fn deserialize() {
    use pretty_assertions::assert_eq;
    use serde_json::json;

    assert_eq!(
        super::Object::Imxkobs(Box::new(Imxkobs {
            filename: "imxkobs-filename".to_string(),
            size: 1024,
            sha256sum: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
                .to_string(),

            install_if_different: None,
            padding_1k: true,
            search_exponent: 2,
            chip_0_device_path: Some(PathBuf::from("/dev/sda1")),
            chip_1_device_path: Some(PathBuf::from("/dev/sda2")),
        })),
        serde_json::from_value::<super::Object>(json!({
            "mode": "imxkobs",
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
