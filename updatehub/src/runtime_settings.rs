// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::serde_helpers::{de, ser};

use chrono::{DateTime, Duration, Utc};
use failure::Fail;
use serde::{Deserialize, Serialize};
use serde_ini;
use slog_scope::debug;
use std::{
    io,
    path::{Path, PathBuf},
};

#[derive(Debug, Default, PartialEq, Deserialize, Serialize)]
#[serde(rename_all = "PascalCase")]
pub(crate) struct RuntimeSettings {
    polling: RuntimePolling,
    update: RuntimeUpdate,
    #[serde(skip)]
    path: PathBuf,
    #[serde(skip)]
    persistent: bool,
}

impl RuntimeSettings {
    pub(crate) fn new() -> Self {
        Self::default()
    }

    pub(crate) fn load(mut self, path: &str) -> Result<Self, failure::Error> {
        use std::{fs::File, io::Read};

        let path = Path::new(path);

        if path.exists() {
            debug!(
                "Loading runtime settings from '{}'...",
                path.to_string_lossy()
            );

            let mut content = String::new();
            File::open(path)?.read_to_string(&mut content)?;
            self = Self::parse(&content)?;
        } else {
            debug!(
                "Runtime settings file {} does not exists. Using default runtime settings...",
                path.to_string_lossy()
            );
        }

        self.path = path.to_path_buf();
        Ok(self)
    }

    fn parse(content: &str) -> Result<Self, failure::Error> {
        Ok(serde_ini::from_str::<Self>(content)?)
    }

    fn save(&self) -> Result<(), failure::Error> {
        use std::{fs::File, io::Write};

        if !self.persistent {
            debug!("Skipping runtime settings save, using non-persistent.");
            return Ok(());
        }

        debug!(
            "Saving runtime settings from '{}'...",
            &self.path.to_string_lossy()
        );

        File::create(&self.path)?.write_all(self.serialize()?.as_bytes())?;
        Ok(())
    }

    fn serialize(&self) -> Result<String, failure::Error> {
        Ok(serde_ini::to_string(&self)?)
    }

    pub(crate) fn enable_persistency(&mut self) {
        self.persistent = true;
    }

    pub(crate) fn is_polling_forced(&self) -> bool {
        self.polling.now
    }

    pub(crate) fn force_poll(&mut self) -> Result<(), failure::Error> {
        self.polling.now = true;
        self.save()
    }

    pub(crate) fn retries(&self) -> usize {
        self.polling.retries
    }

    pub(crate) fn inc_retries(&mut self) {
        self.polling.retries += 1;
    }

    pub(crate) fn clear_retries(&mut self) {
        self.polling.retries = 0;
    }

    pub(crate) fn polling_extra_interval(&self) -> Option<Duration> {
        self.polling.extra_interval
    }

    pub(crate) fn set_polling_extra_interval(
        &mut self,
        extra_interval: Duration,
    ) -> Result<(), failure::Error> {
        self.polling.extra_interval = Some(extra_interval);
        self.save()
    }

    pub(crate) fn last_polling(&self) -> Option<DateTime<Utc>> {
        self.polling.last
    }

    pub(crate) fn set_last_polling(
        &mut self,
        last_polling: DateTime<Utc>,
    ) -> Result<(), failure::Error> {
        self.polling.last = Some(last_polling);
        self.save()
    }

    pub(crate) fn applied_package_uid(&self) -> Option<String> {
        self.update.applied_package_uid.clone()
    }

    pub(crate) fn set_applied_package_uid(
        &mut self,
        applied_package_uid: &str,
    ) -> Result<(), failure::Error> {
        self.update.applied_package_uid = Some(applied_package_uid.to_string());
        self.save()
    }
}

#[derive(Debug, Fail)]
pub(crate) enum Error {
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
struct RuntimePolling {
    #[serde(skip_serializing_if = "Option::is_none")]
    #[serde(rename = "LastPoll")]
    last: Option<DateTime<Utc>>,
    #[serde(deserialize_with = "de::duration_from_int")]
    #[serde(serialize_with = "ser::duration_option_to_int")]
    extra_interval: Option<Duration>,
    retries: usize,
    #[serde(rename = "ProbeASAP")]
    #[serde(deserialize_with = "de::bool_from_str")]
    #[serde(serialize_with = "ser::bool_to_string")]
    now: bool,
}

impl Default for RuntimePolling {
    fn default() -> Self {
        Self {
            last: None,
            extra_interval: None,
            retries: 0,
            now: false,
        }
    }
}

#[derive(Debug, Deserialize, PartialEq, Serialize)]
#[serde(rename_all = "PascalCase")]
struct RuntimeUpdate {
    #[serde(rename = "UpgradeToInstallation")]
    upgrading_to: i8,
    #[serde(skip_serializing_if = "Option::is_none")]
    applied_package_uid: Option<String>,
}

impl Default for RuntimeUpdate {
    fn default() -> Self {
        Self {
            upgrading_to: -1,
            applied_package_uid: None,
        }
    }
}

#[test]
fn de() {
    use pretty_assertions::assert_eq;
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
        update: RuntimeUpdate {
            upgrading_to: 1,
            applied_package_uid: None,
        },
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
    use pretty_assertions::assert_eq;
    let settings = RuntimeSettings::new();
    let expected = RuntimeSettings {
        polling: RuntimePolling {
            last: None,
            extra_interval: None,
            retries: 0,
            now: false,
        },
        update: RuntimeUpdate {
            upgrading_to: -1,
            applied_package_uid: None,
        },
        path: PathBuf::new(),
        persistent: false,
    };

    assert_eq!(Some(settings), Some(expected));
}

#[test]
fn ser() {
    use pretty_assertions::assert_eq;
    let settings = RuntimeSettings {
        polling: RuntimePolling {
            last: Some("2017-01-01T00:00:00Z".parse::<DateTime<Utc>>().unwrap()),
            extra_interval: Some(Duration::seconds(4)),
            retries: 5,
            now: false,
        },
        update: RuntimeUpdate {
            upgrading_to: 1,
            applied_package_uid: Some("package-uid".to_string()),
        },
        ..Default::default()
    };

    assert_eq!(
        serde_ini::from_str(&settings.serialize().unwrap()).ok(),
        Some(settings)
    );
}

#[test]
fn load_and_save() {
    use pretty_assertions::assert_eq;
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
