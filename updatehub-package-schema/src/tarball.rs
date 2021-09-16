// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::definitions::{Filesystem, TargetFormat, TargetType};
use serde::Deserialize;
use std::path::PathBuf;

#[derive(Clone, Deserialize, PartialEq, Debug)]
#[serde(rename_all = "kebab-case")]
pub struct Tarball {
    pub filename: String,
    pub filesystem: Filesystem,
    pub size: u64,
    pub sha256sum: String,
    #[serde(flatten)]
    pub target: TargetType,
    pub target_path: PathBuf,

    #[serde(default)]
    pub compressed: bool,
    #[serde(default)]
    pub required_uncompressed_size: u64,
    #[serde(flatten, default)]
    pub target_format: TargetFormat,
    #[serde(default)]
    pub mount_options: String,
}

#[test]
fn deserialize() {
    use pretty_assertions::assert_eq;
    use serde_json::json;

    assert_eq!(
        super::Object::Tarball(Box::new(Tarball {
            filename: "etc/passwd".to_string(),
            filesystem: Filesystem::Ext4,
            size: 1024,
            sha256sum: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
                .to_string(),
            target: TargetType::Device(std::path::PathBuf::from("/dev/sda")),
            target_path: PathBuf::from("/"),

            compressed: false,
            required_uncompressed_size: 0,
            target_format: TargetFormat::default(),
            mount_options: String::default(),
        })),
        serde_json::from_value::<super::Object>(json!({
            "mode": "tarball",
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
