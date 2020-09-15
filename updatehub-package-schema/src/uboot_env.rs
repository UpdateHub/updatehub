// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use serde::Deserialize;

#[derive(Deserialize, PartialEq, Debug)]
#[serde(rename_all = "kebab-case")]
pub struct UbootEnv {
    pub filename: String,
    pub size: u64,
    pub sha256sum: String,
}

#[test]
fn deserialize() {
    use pretty_assertions::assert_eq;
    use serde_json::json;

    assert_eq!(
        super::Object::UbootEnv(Box::new(UbootEnv {
            filename: "updatehub.defenv".to_string(),
            size: 1024,
            sha256sum: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
                .to_string(),
        })),
        serde_json::from_value::<super::Object>(json!({
            "mode": "uboot-env",
            "filename": "updatehub.defenv",
            "size": 1024,
            "sha256sum": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
        }))
        .unwrap()
    );
}
