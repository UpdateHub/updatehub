// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
//

use Result;

use chrono::{DateTime, Duration, Utc};
use serde_ini;

use std::io;
use std::path::Path;
use std::path::PathBuf;

use serde_helpers::{de, ser};

#[derive(Debug, Default, Deserialize, PartialEq, Serialize)]
#[serde(rename_all = "PascalCase")]
pub struct RuntimeSettings {
    pub polling: RuntimePolling,
    pub update: RuntimeUpdate,
    #[serde(skip)]
    path: PathBuf,
}

impl RuntimeSettings {
    pub fn new() -> Self {
        RuntimeSettings::default()
    }

    pub fn load(mut self, path: &str) -> Result<Self> {
        use std::fs::File;
        use std::io::Read;

        let path = Path::new(path);

        if path.exists() {
            info!(
                "Loading runtime settings from '{}'...",
                path.to_string_lossy()
            );

            let mut content = String::new();
            File::open(path)?.read_to_string(&mut content)?;
            self = RuntimeSettings::parse(&content)?;
        } else {
            debug!(
                "Runtime settings file {} does not exists.",
                path.to_string_lossy()
            );
            info!("Using default runtime settings...");
        }

        self.path = path.to_path_buf();
        Ok(self)
    }

    fn parse(content: &str) -> Result<Self> {
        Ok(serde_ini::from_str::<RuntimeSettings>(content)?)
    }

    pub fn save(&self) -> Result<usize> {
        use std::fs::File;
        use std::io::Write;

        debug!(
            "Saving runtime settings from '{}'...",
            &self.path.to_string_lossy()
        );

        Ok(File::create(&self.path)?.write(self.serialize()?.as_bytes())?)
    }

    fn serialize(&self) -> Result<String> {
        Ok(serde_ini::to_string(&self)?)
    }
}

#[derive(Debug, Fail)]
pub enum RuntimeSettingsError {
    #[cause]
    #[fail(display = "IO error")]
    Io(io::Error),
    #[cause]
    #[fail(display = "Fail reading the INI file")]
    IniDeserialize(serde_ini::de::Error),
    #[cause]
    #[fail(display = "Fail generating the INI file")]
    IniSerialize(serde_ini::ser::Error),
}

#[derive(Debug, Deserialize, PartialEq, Serialize)]
#[serde(rename_all = "PascalCase")]
pub struct RuntimePolling {
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "LastPoll")]
    pub last: Option<DateTime<Utc>>,
    #[serde(deserialize_with = "de::duration_from_int")]
    #[serde(serialize_with = "ser::duration_to_int")]
    pub extra_interval: Option<Duration>,
    pub retries: usize,
    #[serde(rename = "ProbeASAP")]
    #[serde(deserialize_with = "de::bool_from_str")]
    #[serde(serialize_with = "ser::bool_to_string")]
    pub now: bool,
}

impl Default for RuntimePolling {
    fn default() -> Self {
        RuntimePolling {
            last: None,
            extra_interval: None,
            retries: 0,
            now: false,
        }
    }
}

#[derive(Debug, Deserialize, PartialEq, Serialize)]
#[serde(rename_all = "PascalCase")]
pub struct RuntimeUpdate {
    #[serde(rename = "UpgradeToInstallation")]
    pub upgrading_to: i8,
}

impl Default for RuntimeUpdate {
    fn default() -> Self {
        RuntimeUpdate { upgrading_to: -1 }
    }
}

#[test]
fn de() {
    let ini = r"
[Polling]
LastPoll=2017-01-01T00:00:00Z
ExtraInterval=4
Retries=5
ProbeASAP=false

[Update]
UpgradeToInstallation=1
";
    let expected = RuntimeSettings {
        polling: RuntimePolling {
            last: Some("2017-01-01T00:00:00Z".parse::<DateTime<Utc>>().unwrap()),
            extra_interval: Some(Duration::seconds(4)),
            retries: 5,
            now: false,
        },
        update: RuntimeUpdate { upgrading_to: 1 },
        ..Default::default()
    };

    assert_eq!(
        serde_ini::from_str::<RuntimeSettings>(ini)
            .map_err(|e| println!("{}", e))
            .as_ref()
            .ok(),
        Some(&expected)
    );
    assert_eq!(RuntimeSettings::parse(ini).as_ref().ok(), Some(&expected));
}

#[test]
fn default() {
    let settings = RuntimeSettings::new();
    let expected = RuntimeSettings {
        polling: RuntimePolling {
            last: None,
            extra_interval: None,
            retries: 0,
            now: false,
        },
        update: RuntimeUpdate { upgrading_to: -1 },
        path: PathBuf::new(),
    };

    assert_eq!(Some(settings), Some(expected));
}

#[test]
fn ser() {
    let settings = RuntimeSettings {
        polling: RuntimePolling {
            last: Some("2017-01-01T00:00:00Z".parse::<DateTime<Utc>>().unwrap()),
            extra_interval: Some(Duration::seconds(4)),
            retries: 5,
            now: false,
        },
        update: RuntimeUpdate { upgrading_to: 1 },
        ..Default::default()
    };

    assert_eq!(
        serde_ini::from_str(&settings.serialize().ok().unwrap())
            .map_err(|e| println!("{}", e))
            .ok(),
        Some(settings)
    );
}

#[test]
fn load_and_save() {
    use std::fs;
    use tempfile::NamedTempFile;

    let tempfile = NamedTempFile::new().unwrap();
    let settings_file = tempfile.path();
    fs::remove_file(&settings_file).unwrap();

    let mut settings = RuntimeSettings::new()
        .load(settings_file.to_str().unwrap())
        .unwrap();

    assert_eq!(settings.polling.now, false);
    settings.polling.now = true;

    assert_eq!(settings.polling.now, true);
    settings
        .save()
        .expect("Failed to save the runtime settings");

    let new_settings = RuntimeSettings::new()
        .load(settings_file.to_str().unwrap())
        .unwrap();

    assert_eq!(settings.update, new_settings.update);
}
