/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contact@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

use serde::{de, Deserialize, Deserializer};

use std::time::Duration;

pub fn duration_from_str<'de, D>(deserializer: D) -> Result<Duration, D::Error>
    where D: Deserializer<'de> {
    use parse_duration::parse;

    let s = String::deserialize(deserializer)?;
    parse(&s).map_err(de::Error::custom)
}

pub fn bool_from_str<'de, D>(deserializer: D) -> Result<bool, D::Error>
    where D: Deserializer<'de> {
    use std::str::FromStr;

    let s = String::deserialize(deserializer)?;
    bool::from_str(&s).map_err(de::Error::custom)
}

pub fn vec_from_str<'de, D>(deserializer: D) -> Result<Vec<String>, D::Error>
    where D: Deserializer<'de> {
    Ok(String::deserialize(deserializer)?.split(',')
                                         .map(|s| s.to_string())
                                         .collect())
}
