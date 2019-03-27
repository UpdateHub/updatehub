// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use serde::Deserialize;

#[derive(PartialEq, Debug, Deserialize)]
#[serde(rename_all = "kebab-case")]
pub struct TargetPermissions {
    //FIXME: Process string input into usize
    pub target_mode: Option<String>,
    pub target_gid: Option<Id>,
    pub target_uid: Option<Id>,
}

#[derive(PartialEq, Debug, Deserialize)]
#[serde(untagged)]
pub enum Id {
    Name(String),
    Number(usize),
}

#[test]
fn deserialize() {
    use pretty_assertions::assert_eq;
    use serde_json::json;

    assert_eq!(
        TargetPermissions {
            target_mode: Some("0777".to_string()),
            target_gid: Some(Id::Name("wheel".to_string())),
            target_uid: Some(Id::Name("user".to_string())),
        },
        serde_json::from_value::<TargetPermissions>(json!({
            "target-mode": "0777",
            "target-uid": "user",
            "target-gid": "wheel"
        }))
        .unwrap()
    );

    assert_eq!(
        TargetPermissions {
            target_mode: None,
            target_gid: Some(Id::Number(1000)),
            target_uid: Some(Id::Number(1000)),
        },
        serde_json::from_value::<TargetPermissions>(json!({
            "target-uid": 1000,
            "target-gid": 1000,
        }))
        .unwrap()
    );
}
