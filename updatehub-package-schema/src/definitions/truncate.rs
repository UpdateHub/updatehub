// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use serde::Deserialize;

/// True if the file pointed to by the `target_path` should be open in
/// truncate mode (erase content before writing).
#[derive(PartialEq, Debug, Deserialize)]
pub struct Truncate(pub bool);

impl Default for Truncate {
    fn default() -> Self {
        Truncate(true)
    }
}

#[cfg(test)]
mod test {
    use super::*;
    use pretty_assertions::assert_eq;
    use serde_json::json;

    #[derive(Debug, PartialEq, Deserialize)]
    struct Payload {
        #[serde(default)]
        truncate: Truncate,
    }

    #[test]
    fn deserialize() {
        assert_eq!(
            serde_json::from_value::<Payload>(json!({ "truncate": false })).ok(),
            Some(Payload { truncate: Truncate(false) })
        );
    }

    #[test]
    fn default() {
        assert_eq!(
            serde_json::from_value::<Payload>(json!({})).ok(),
            Some(Payload { truncate: Truncate(true) })
        );
    }
}
