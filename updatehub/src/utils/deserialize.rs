// Copyright (C) 2019-2021 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use chrono::Duration;
use serde::{Deserialize, Deserializer, de};

pub(crate) fn duration<'de, D>(deserializer: D) -> Result<Duration, D::Error>
where
    D: Deserializer<'de>,
{
    use ms_converter::ms;

    let s = String::deserialize(deserializer)?;
    Ok(Duration::milliseconds(ms(s).map_err(de::Error::custom)?))
}

pub fn boolean<'de, D>(deserializer: D) -> Result<bool, D::Error>
where
    D: Deserializer<'de>,
{
    use std::str::FromStr;

    let s = String::deserialize(deserializer)?;
    bool::from_str(&s).map_err(de::Error::custom)
}

pub fn string_list<'de, D>(deserializer: D) -> Result<Vec<String>, D::Error>
where
    D: Deserializer<'de>,
{
    Ok(String::deserialize(deserializer)?
        .split(',')
        .map(std::string::ToString::to_string)
        .collect())
}
