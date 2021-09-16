// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::definitions::{
    Filesystem, InstallIfDifferent, TargetFormat, TargetPermissions, TargetType,
};
use serde::Deserialize;
use std::path::PathBuf;

#[derive(Clone, Deserialize, PartialEq, Debug)]
#[serde(rename_all = "kebab-case")]
pub struct Copy {
    pub filename: String,
    pub filesystem: Filesystem,
    pub size: u64,
    pub sha256sum: String,
    #[serde(flatten)]
    pub target_type: TargetType,
    pub target_path: PathBuf,

    pub install_if_different: Option<InstallIfDifferent>,
    #[serde(flatten)]
    pub target_permissions: TargetPermissions,
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
        super::Object::Copy(Box::new(Copy {
            filename: "etc/passwd".to_string(),
            filesystem: Filesystem::Btrfs,
            size: 1024,
            sha256sum: "cfe2be1c64b0387500853de0f48303e3de7b1c6f1508dc719eeafa0d41c36722"
                .to_string(),
            target_type: TargetType::Device(PathBuf::from("/dev/sda")),
            target_path: PathBuf::from("/etc/passwd"),

            install_if_different: Some(InstallIfDifferent::CheckSum),
            target_permissions: TargetPermissions::default(),
            compressed: false,
            required_uncompressed_size: 0,
            target_format: TargetFormat::default(),
            mount_options: String::default(),
        })),
        serde_json::from_value::<super::Object>(json!({
            "mode": "copy",
            "filename": "etc/passwd",
            "size": 1024,
            "sha256sum": "cfe2be1c64b0387500853de0f48303e3de7b1c6f1508dc719eeafa0d41c36722",
            "install-if-different": "sha256sum",
            "filesystem": "btrfs",
            "target-type": "device",
            "target": "/dev/sda",
            "target-path": "/etc/passwd"
        }))
        .unwrap()
    );
}
