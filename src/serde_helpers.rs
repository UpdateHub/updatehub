//
// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: GPL-2.0
//

pub mod ser {
    use serde::Serializer;

    pub fn bool_to_string<S>(v: &bool, serializer: S) -> Result<S::Ok, S::Error>
        where S: Serializer {
        Ok(serializer.serialize_str(if *v { "true" } else { "false" })?)
    }
}

pub mod de {
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
}
