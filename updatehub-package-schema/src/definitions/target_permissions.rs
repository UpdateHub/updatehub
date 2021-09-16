// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use serde::{de, Deserialize, Deserializer};

/// Options to set permissions after installing on target.
#[derive(Clone, PartialEq, Debug, Deserialize, Default)]
#[serde(rename_all = "kebab-case")]
#[serde(default)]
pub struct TargetPermissions {
    #[serde(deserialize_with = "optional_octal_from_str")]
    pub target_mode: Option<u32>,
    pub target_gid: Option<Gid>,
    pub target_uid: Option<Uid>,
}

#[derive(Clone, PartialEq, Debug, Deserialize)]
#[serde(untagged)]
pub enum Gid {
    /// Group name.
    Name(String),

    /// Group numeric id.
    Number(u32),
}

#[derive(Clone, PartialEq, Debug, Deserialize)]
#[serde(untagged)]
pub enum Uid {
    /// User name.
    Name(String),

    /// User numeric id.
    Number(u32),
}

fn optional_octal_from_str<'de, D>(deserializer: D) -> Result<Option<u32>, D::Error>
where
    D: Deserializer<'de>,
{
    Option::<String>::deserialize(deserializer).and_then(|opt| {
        opt.map(|s| u32::from_str_radix(&s, 8).map_err(de::Error::custom)).transpose()
    })
}

#[test]
fn deserialize() {
    use pretty_assertions::assert_eq;
    use serde_json::json;

    assert_eq!(
        TargetPermissions {
            target_mode: Some(0o0777),
            target_gid: Some(Gid::Name("wheel".to_string())),
            target_uid: Some(Uid::Name("user".to_string())),
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
            target_gid: Some(Gid::Number(1000)),
            target_uid: Some(Uid::Number(1000)),
        },
        serde_json::from_value::<TargetPermissions>(json!({
            "target-uid": 1000,
            "target-gid": 1000,
        }))
        .unwrap()
    );
}
