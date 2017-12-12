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

use std::io;

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

    pub fn load(self, path: &str) -> Result<Self, PersistentSettingsError> {
        use std::fs::File;
        use std::io::Read;
        use std::path::Path;

        let path = Path::new(path);

        if path.exists() {
            info!(
                "Loading persistent settings from '{}'...",
                path.to_string_lossy()
            );

            let mut content = String::new();
            File::open(path)?.read_to_string(&mut content)?;

            Ok(self.parse(&content)?)
        } else {
            debug!(
                "Persistent settings file {} does not exists.",
                path.to_string_lossy()
            );
            info!("Using default persistent settings...");
            Ok(self)
        }
    }

    fn parse(self, content: &str) -> Result<Self, PersistentSettingsError> {
        Ok(serde_ini::from_str::<PersistentSettings>(&content)?)
    }
}

#[derive(Debug)]
pub enum PersistentSettingsError {
    Io(io::Error),
    Ini(serde_ini::de::Error),
}

impl From<io::Error> for PersistentSettingsError {
    fn from(err: io::Error) -> PersistentSettingsError {
        PersistentSettingsError::Io(err)
    }
}

impl From<serde_ini::de::Error> for PersistentSettingsError {
    fn from(err: serde_ini::de::Error) -> PersistentSettingsError {
        PersistentSettingsError::Ini(err)
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
                .as_ref()
                .ok() == Some(&expected)
        );
        assert!(PersistentSettings::new().parse(&ini).as_ref().ok() == Some(&expected));
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
