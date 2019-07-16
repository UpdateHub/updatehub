// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use serde::{de, Deserialize, Deserializer};

/// How many `ChunkSize` blocks must be copied from the source file to
/// the target. The default value of -1 means all possible bytes
/// until the end of the file
#[derive(PartialEq, Debug, Clone)]
pub enum Count {
    All,
    Limited(usize),
}

impl Default for Count {
    fn default() -> Self {
        Count::All
    }
}

impl<'de> Deserialize<'de> for Count {
    fn deserialize<D>(deserializer: D) -> Result<Self, D::Error>
    where
        D: Deserializer<'de>,
    {
        match isize::deserialize(deserializer)? {
            -1 => Ok(Count::All),
            n if n >= 0 => Ok(Count::Limited(n as usize)),
            n => Err(de::Error::custom(format!("Invalid count: {}", n))),
        }
    }
}

impl std::iter::Iterator for Count {
    type Item = usize;

    fn next(&mut self) -> Option<Self::Item> {
        match self {
            Count::All => Some(0),
            Count::Limited(ref mut n) => match n {
                0 => None,
                n => {
                    *n -= 1;
                    Some(*n)
                }
            },
        }
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
        count: Count,
    }

    #[test]
    fn deserialize() {
        assert_eq!(
            serde_json::from_value::<Payload>(json!({ "count": 0 })).unwrap(),
            Payload {
                count: Count::Limited(0)
            }
        );
    }

    #[test]
    fn default() {
        assert_eq!(
            serde_json::from_value::<Payload>(json!({})).unwrap(),
            Payload { count: Count::All }
        );
    }

    #[test]
    fn validation_of_minimal() {
        assert!(serde_json::from_value::<Payload>(json!({ "count": -2 })).is_err());
    }

}
