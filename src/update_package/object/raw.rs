// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{definitions, ObjectType};
use serde::Deserialize;

#[derive(Deserialize, PartialEq, Debug)]
#[serde(rename_all = "kebab-case")]
pub(crate) struct Raw {
    filename: String,
    size: u64,
    sha256sum: String,
    #[serde(flatten)]
    target_type: definitions::TargetType,

    install_if_different: Option<definitions::InstallIfDifferent>,
    #[serde(default)]
    compressed: bool,
    #[serde(default)]
    required_uncompressed_size: u64,
    #[serde(default)]
    chunk_size: definitions::ChunkSize,
    #[serde(default)]
    skip: definitions::Skip,
    #[serde(default)]
    seek: u64,
    #[serde(default)]
    count: definitions::Count,
    #[serde(default)]
    truncate: definitions::Truncate,
}

impl_object_type!(Raw);

#[test]
fn deserialize() {
    use pretty_assertions::assert_eq;
    use serde_json::json;
    use std::path::PathBuf;

    assert_eq!(
        Raw {
            filename: "etc/passwd".to_string(),
            size: 1024,
            sha256sum: "cfe2be1c64b0387500853de0f48303e3de7b1c6f1508dc719eeafa0d41c36722"
                .to_string(),
            target_type: definitions::TargetType::Device(PathBuf::from("/dev/sdb")),

            install_if_different: Some(definitions::InstallIfDifferent::CheckSum(
                definitions::install_if_different::CheckSum::Sha256Sum
            )),
            compressed: true,
            required_uncompressed_size: 2048,
            chunk_size: definitions::ChunkSize::default(),
            skip: definitions::Skip::default(),
            seek: u64::default(),
            count: definitions::Count::default(),
            truncate: definitions::Truncate::default(),
        },
        serde_json::from_value::<Raw>(json!({
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
