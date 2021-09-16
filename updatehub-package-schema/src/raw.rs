// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::definitions::{ChunkSize, Count, InstallIfDifferent, Skip, TargetType, Truncate};
use serde::Deserialize;

#[derive(Clone, Deserialize, PartialEq, Debug)]
#[serde(rename_all = "kebab-case")]
pub struct Raw {
    pub filename: String,
    pub size: u64,
    pub sha256sum: String,
    #[serde(flatten)]
    pub target_type: TargetType,

    pub install_if_different: Option<InstallIfDifferent>,
    #[serde(default)]
    pub compressed: bool,
    #[serde(default)]
    pub required_uncompressed_size: u64,
    #[serde(default)]
    pub chunk_size: ChunkSize,
    #[serde(default)]
    pub skip: Skip,
    #[serde(default)]
    pub seek: u64,
    #[serde(default)]
    pub count: Count,
    #[serde(default)]
    pub truncate: Truncate,
}

#[test]
fn deserialize() {
    use pretty_assertions::assert_eq;
    use serde_json::json;
    use std::path::PathBuf;

    assert_eq!(
        super::Object::Raw(Box::new(Raw {
            filename: "etc/passwd".to_string(),
            size: 1024,
            sha256sum: "cfe2be1c64b0387500853de0f48303e3de7b1c6f1508dc719eeafa0d41c36722"
                .to_string(),
            target_type: TargetType::Device(PathBuf::from("/dev/sdb")),

            install_if_different: Some(InstallIfDifferent::CheckSum),
            compressed: true,
            required_uncompressed_size: 2048,
            chunk_size: ChunkSize::default(),
            skip: Skip::default(),
            seek: u64::default(),
            count: Count::default(),
            truncate: Truncate::default(),
        })),
        serde_json::from_value::<super::Object>(json!({
            "mode": "raw",
            "filename": "etc/passwd",
            "size": 1024,
            "sha256sum": "cfe2be1c64b0387500853de0f48303e3de7b1c6f1508dc719eeafa0d41c36722",
            "install-if-different": "sha256sum",
            "target-type": "device",
            "target": "/dev/sdb",
            "compressed": true,
            "required-uncompressed-size": 2048
        }))
        .unwrap()
    );
}
