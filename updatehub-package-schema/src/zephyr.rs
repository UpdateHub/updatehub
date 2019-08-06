// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use serde::Deserialize;

#[derive(Deserialize, PartialEq, Debug)]
pub struct Zephyr {
    pub filename: String,
    pub size: u64,
    pub sha256sum: String,
}

#[test]
fn deserialize() {
    use pretty_assertions::assert_eq;
    use serde_json::json;

    assert_eq!(
        Zephyr {
            filename: "artifact.zephyr".to_string(),
            size: 1024,
            sha256sum: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
                .to_string(),
        },
        serde_json::from_value::<Zephyr>(json!({
            "filename": "artifact.zephyr",
            "size": 1024,
            "sha256sum": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
        }))
        .unwrap()
    );
}
