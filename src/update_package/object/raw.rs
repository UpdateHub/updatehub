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
    target: definitions::TargetType,

    install_if_different: Option<definitions::InstallIfDifferent>,
    compressed: Option<bool>,
    required_uncompressed_size: Option<u64>,
    chunk_size: Option<definitions::ChunkSize>,
    skip: Option<definitions::Skip>,
    seek: Option<u64>,
    count: Option<definitions::Count>,
    truncate: Option<definitions::Truncate>,
}

impl_object_type!(Raw);

#[test]
fn deserialize() {
    use serde_json::json;

    assert_eq!(
        Raw {
            filename: "etc/passwd".to_string(),
            size: 1024,
            sha256sum: "cfe2be1c64b0387500853de0f48303e3de7b1c6f1508dc719eeafa0d41c36722"
                .to_string(),
            target: definitions::TargetType::Device("/dev/sdb".to_string()),

            install_if_different: Some(definitions::InstallIfDifferent::CheckSum(
                definitions::install_if_different::CheckSum::Sha256Sum
            )),
            compressed: Some(true),
            required_uncompressed_size: Some(2048),
            chunk_size: None,
            skip: None,
            seek: None,
            count: None,
            truncate: None,
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
