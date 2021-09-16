// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use serde::Deserialize;

#[derive(Clone, Deserialize, PartialEq, Debug)]
pub struct Mender {
    pub filename: String,
    pub size: u64,
    pub sha256sum: String,
}

#[test]
fn deserialize() {
    use pretty_assertions::assert_eq;
    use serde_json::json;

    assert_eq!(
        super::Object::Mender(Box::new(Mender {
            filename: "artifact.mender".to_string(),
            size: 1024,
            sha256sum: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
                .to_string(),
        })),
        serde_json::from_value::<super::Object>(json!({
            "mode": "mender",
            "filename": "artifact.mender",
            "size": 1024,
            "sha256sum": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
        }))
        .unwrap()
    );
}
