// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

#![allow(clippy::trivially_copy_pass_by_ref)]
pub mod ser {
    use chrono::Duration;
    use serde::Serializer;

    pub fn bool_to_string<S>(v: &bool, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: Serializer,
    {
        serializer.serialize_str(if *v { "true" } else { "false" })
    }

    pub fn duration_option_to_int<S>(v: &Option<Duration>, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: Serializer,
    {
        serializer.serialize_i64(v.unwrap_or(Duration::seconds(0)).num_seconds())
    }

    pub fn duration_to_int<S>(v: &Duration, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: Serializer,
    {
        serializer.serialize_i64(v.num_seconds())
    }
}

pub mod de {
    use chrono::Duration;
    use serde::{de, Deserialize, Deserializer};

    pub fn duration_from_str<'de, D>(deserializer: D) -> Result<Duration, D::Error>
    where
        D: Deserializer<'de>,
    {
        use parse_duration::parse;

        let s = String::deserialize(deserializer)?;
        Ok(Duration::from_std(parse(&s).map_err(de::Error::custom)?).map_err(de::Error::custom)?)
    }

    pub fn octal_from_str<'de, D>(deserializer: D) -> Result<u32, D::Error>
    where
        D: Deserializer<'de>,
    {
        let s = String::deserialize(deserializer)?;
        Ok(u32::from_str_radix(&s, 8).map_err(de::Error::custom)?)
    }

    pub fn duration_from_int<'de, D>(deserializer: D) -> Result<Option<Duration>, D::Error>
    where
        D: Deserializer<'de>,
    {
        let i = i64::deserialize(deserializer)?;
        Ok(if i > 0 {
            Some(Duration::seconds(i))
        } else {
            None
        })
    }

    pub fn bool_from_str<'de, D>(deserializer: D) -> Result<bool, D::Error>
    where
        D: Deserializer<'de>,
    {
        use std::str::FromStr;

        let s = String::deserialize(deserializer)?;
        bool::from_str(&s).map_err(de::Error::custom)
    }

    pub fn vec_from_str<'de, D>(deserializer: D) -> Result<Vec<String>, D::Error>
    where
        D: Deserializer<'de>,
    {
        Ok(String::deserialize(deserializer)?
            .split(',')
            .map(std::string::ToString::to_string)
            .collect())
    }

    pub fn supported_hardware_any<'de, D>(deserializer: D) -> Result<(), D::Error>
    where
        D: Deserializer<'de>,
    {
        if String::deserialize(deserializer)? == "any" {
            Ok(())
        } else {
            Err(de::Error::custom("expected \"any\""))
        }
    }
}
