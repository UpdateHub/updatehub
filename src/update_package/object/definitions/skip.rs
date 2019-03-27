// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use serde::Deserialize;

/// How many chunk-size blocks must be skipped in the source file
#[derive(PartialEq, Debug, Default, Deserialize)]
pub struct Skip(#[serde(deserialize_with = "usize::deserialize")] usize);

#[cfg(test)]
mod test {
    use super::*;
    use pretty_assertions::assert_eq;
    use serde_json::json;

    #[derive(Debug, PartialEq, Deserialize)]
    struct Payload {
        #[serde(default)]
        skip: Skip,
    }

    #[test]
    fn deserialize() {
        assert_eq!(
            serde_json::from_value::<Payload>(json!({ "skip": 10 })).ok(),
            Some(Payload { skip: Skip(10) })
        );
    }

    #[test]
    fn default() {
        assert_eq!(
            serde_json::from_value::<Payload>(json!({})).ok(),
            Some(Payload { skip: Skip(0) })
        );
    }
}
