// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

pub(crate) mod duration_option {
    use chrono::Duration;
    use serde::{Deserialize, Deserializer, Serializer};

    pub(crate) fn serialize<S>(v: &Option<Duration>, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: Serializer,
    {
        serializer.serialize_i64(v.unwrap_or(Duration::seconds(0)).num_seconds())
    }

    pub(crate) fn deserialize<'de, D>(deserializer: D) -> Result<Option<Duration>, D::Error>
    where
        D: Deserializer<'de>,
    {
        let i = i64::deserialize(deserializer)?;
        Ok(if i > 0 { Some(Duration::seconds(i)) } else { None })
    }
}

pub(crate) mod duration {
    use chrono::Duration;
    use serde::{de, Deserialize, Deserializer, Serializer};

    pub(crate) fn serialize<S>(v: &Duration, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: Serializer,
    {
        serializer.serialize_str(&format!("{}s", v.num_seconds()))
    }

    pub(crate) fn deserialize<'de, D>(deserializer: D) -> Result<Duration, D::Error>
    where
        D: Deserializer<'de>,
    {
        use parse_duration::parse;

        let s = String::deserialize(deserializer)?;
        Ok(Duration::from_std(parse(&s).map_err(de::Error::custom)?).map_err(de::Error::custom)?)
    }
}
