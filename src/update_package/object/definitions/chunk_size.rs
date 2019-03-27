// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use serde::{de, Deserialize, Deserializer};

/// The size of the buffers (in bytes) used to read and write,
/// default is the 128KiB
#[derive(PartialEq, Debug)]
pub struct ChunkSize(usize);

impl Default for ChunkSize {
    fn default() -> Self {
        ChunkSize(131_072)
    }
}

impl<'de> Deserialize<'de> for ChunkSize {
    fn deserialize<D>(deserializer: D) -> Result<Self, D::Error>
    where
        D: Deserializer<'de>,
    {
        let n = usize::deserialize(deserializer)?;
        if n > 1 {
            return Ok(ChunkSize(n));
        }
        Err(de::Error::custom(format!("Invalid chunk size: {}", n)))
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
        chunk_size: ChunkSize,
    }

    #[test]
    fn deserialize() {
        assert_eq!(
            serde_json::from_value::<Payload>(json!({ "chunk_size": 313 })).ok(),
            Some(Payload {
                chunk_size: ChunkSize(313)
            })
        );
        assert!(serde_json::from_value::<Payload>(json!({ "chunk_size": 0 })).is_err())
    }

    #[test]
    fn default() {
        assert_eq!(
            serde_json::from_value::<Payload>(json!({})).ok(),
            Some(Payload {
                chunk_size: ChunkSize(131_072)
            })
        );
    }
}
