// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{definitions, ObjectType};
use serde::Deserialize;

#[derive(Deserialize, PartialEq, Debug)]
#[serde(rename_all = "kebab-case")]
pub(crate) struct Tarball {
    filename: String,
    filesystem: definitions::Filesystem,
    size: u64,
    sha256sum: String,
    #[serde(flatten)]
    target: definitions::TargetType,
    target_path: String,

    compressed: Option<bool>,
    required_uncompressed_size: Option<u64>,
    #[serde(flatten)]
    target_format: Option<definitions::TargetFormat>,
    mount_options: Option<String>,
}

impl_object_type!(Tarball);

#[test]
fn deserialize() {
    use serde_json::json;

    assert_eq!(
        Tarball {
            filename: "etc/passwd".to_string(),
            filesystem: definitions::Filesystem::Ext4,
            size: 1024,
            sha256sum: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
                .to_string(),
            target: definitions::TargetType::Device("/dev/sda".to_string()),
            target_path: "/".to_string(),

            compressed: None,
            required_uncompressed_size: None,
            target_format: None,
            mount_options: None,
        },
        serde_json::from_value::<Tarball>(json!({
            "filename": "etc/passwd",
            "size": 1024,
            "sha256sum": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
            "target-type": "device",
            "target": "/dev/sda",
            "filesystem": "ext4",
            "target-path": "/"
        }))
        .unwrap()
    );
}
