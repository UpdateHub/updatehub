// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{definitions, ObjectType};
use serde::Deserialize;

#[derive(Deserialize, PartialEq, Debug)]
#[serde(rename_all = "kebab-case")]
pub(crate) struct Copy {
    filename: String,
    filesystem: definitions::Filesystem,
    size: u64,
    sha256sum: String,
    #[serde(flatten)]
    target: definitions::TargetType,
    target_path: String,

    install_if_different: Option<definitions::InstallIfDifferent>,
    #[serde(flatten)]
    target_permissions: definitions::TargetPermissions,
    compressed: Option<bool>,
    required_uncompressed_size: Option<u64>,
    #[serde(flatten)]
    target_format: Option<definitions::TargetFormat>,
    mount_options: Option<String>,
}

impl_object_type!(Copy);

#[test]
fn deserialize() {
    use pretty_assertions::assert_eq;
    use serde_json::json;

    assert_eq!(
        Copy {
            filename: "etc/passwd".to_string(),
            filesystem: definitions::Filesystem::Btrfs,
            size: 1024,
            sha256sum: "cfe2be1c64b0387500853de0f48303e3de7b1c6f1508dc719eeafa0d41c36722"
                .to_string(),
            target: definitions::TargetType::Device("/dev/sda".to_string()),
            target_path: "/etc/passwd".to_string(),

            install_if_different: Some(definitions::InstallIfDifferent::CheckSum(
                definitions::install_if_different::CheckSum::Sha256Sum
            )),
            target_permissions: definitions::TargetPermissions {
                target_mode: None,
                target_gid: None,
                target_uid: None,
            },
            compressed: None,
            required_uncompressed_size: None,
            target_format: None,
            mount_options: None,
        },
        serde_json::from_value::<Copy>(json!({
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
