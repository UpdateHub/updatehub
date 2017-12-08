/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contact@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

use serde_ini;

extern crate chrono;
use self::chrono::{DateTime, Utc};

use de_helpers::bool_from_str;

#[derive(Default, Deserialize, PartialEq, Serialize)]
#[serde(rename_all = "PascalCase")]
pub struct PersistentSettings {
    pub polling: PersistentPolling,
    pub update: PersistentUpdate,
}

impl PersistentSettings {
    pub fn new() -> Self {
        PersistentSettings::default()
    }
}

#[derive(Deserialize, PartialEq, Serialize)]
#[serde(rename_all = "PascalCase")]
pub struct PersistentPolling {
    #[serde(rename = "LastPoll")]
    pub last: DateTime<Utc>,
    #[serde(rename = "FirstPoll")]
    pub first: DateTime<Utc>,
    pub extra_interval: usize,
    pub retries: usize,
    #[serde(rename = "ProbeASAP")]
    #[serde(deserialize_with = "bool_from_str")]
    pub now: bool,
}

impl Default for PersistentPolling {
    fn default() -> Self {
        PersistentPolling {
            last: Utc::now(),
            first: Utc::now(),
            extra_interval: 0,
            retries: 0,
            now: false,
        }
    }
}

#[derive(Deserialize, PartialEq, Serialize)]
#[serde(rename_all = "PascalCase")]
pub struct PersistentUpdate {
    #[serde(rename = "UpgradeToInstallation")]
    pub upgrading_to: i8,
}

impl Default for PersistentUpdate {
    fn default() -> Self {
        PersistentUpdate { upgrading_to: -1 }
    }
}

#[cfg(test)]
mod ini_de {
    use super::*;

    #[test]
    fn ok() {
        let ini = r"
[Polling]
LastPoll=2017-01-01T00:00:00Z
FirstPoll=2017-02-02T00:00:00Z
ExtraInterval=4
Retries=5
ProbeASAP=false

[Update]
UpgradeToInstallation=1
";
        let expected = PersistentSettings {
            polling: PersistentPolling {
                last: "2017-01-01T00:00:00Z".parse::<DateTime<Utc>>().unwrap(),
                first: "2017-02-02T00:00:00Z".parse::<DateTime<Utc>>().unwrap(),
                extra_interval: 4,
                retries: 5,
                now: false,
            },
            update: PersistentUpdate { upgrading_to: 1 },
        };

        assert!(
            serde_ini::from_str::<PersistentSettings>(&ini)
                .map_err(|e| println!("{}", e))
                .ok() == Some(expected)
        );
    }

    #[test]
    fn default() {
        let settings = PersistentSettings::new();
        let expected = PersistentSettings {
            polling: PersistentPolling {
                last: settings.polling.last,
                first: settings.polling.first,
                extra_interval: 0,
                retries: 0,
                now: false,
            },
            update: PersistentUpdate { upgrading_to: -1 },
        };

        assert!(Some(settings) == Some(expected));
    }
}
