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

    #[serde(default)]
    compressed: bool,
    #[serde(default)]
    required_uncompressed_size: u64,
    #[serde(flatten, default)]
    target_format: definitions::TargetFormat,
    #[serde(default)]
    mount_options: String,
}

impl_object_type!(Tarball);

#[test]
fn deserialize() {
    use pretty_assertions::assert_eq;
    use serde_json::json;

    assert_eq!(
        Tarball {
            filename: "etc/passwd".to_string(),
            filesystem: definitions::Filesystem::Ext4,
            size: 1024,
            sha256sum: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
                .to_string(),
            target: definitions::TargetType::Device(std::path::PathBuf::from("/dev/sda")),
            target_path: "/".to_string(),

            compressed: bool::default(),
            required_uncompressed_size: u64::default(),
            target_format: definitions::TargetFormat::default(),
            mount_options: String::default(),
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
