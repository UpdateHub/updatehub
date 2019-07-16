// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use serde::Deserialize;

#[derive(PartialEq, Debug, Deserialize, Default)]
#[serde(rename_all = "kebab-case")]
pub struct TargetFormat {
    #[serde(rename = "format?", default)]
    pub should_format: bool,
    pub format_options: Option<String>,
}

#[test]
fn deserialize() {
    use pretty_assertions::assert_eq;
    use serde_json::json;

    assert_eq!(
        TargetFormat {
            should_format: true,
            format_options: Some("-fs ext2".to_string()),
        },
        serde_json::from_value::<TargetFormat>(json!({
            "format?": true,
            "format-options": "-fs ext2"
        }))
        .unwrap()
    );

    assert_eq!(
        TargetFormat {
            should_format: false,
            format_options: None,
        },
        serde_json::from_value::<TargetFormat>(json!({
            "format?": false,
        }))
        .unwrap()
    );
}

#[test]
fn default() {
    use pretty_assertions::assert_eq;

    assert_eq!(
        TargetFormat {
            should_format: false,
            format_options: None,
        },
        TargetFormat::default(),
    );
}
