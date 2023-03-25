// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::firmware::{
    self,
    installation_set::{self, Set},
};
use chrono::{DateTime, NaiveDateTime, Utc};
use derive_more::{Deref, DerefMut, Display, Error, From};
use sdk::api::info::runtime_settings as api;
use slog_scope::{debug, warn};
use std::{fs, io, path::Path};

pub type Result<T> = std::result::Result<T, Error>;

#[derive(Debug, Display, Error, From)]
pub enum Error {
    Io(io::Error),
    SerdeJson(serde_json::Error),
    Firmware(firmware::Error),

    #[display(fmt = "invalid runtime settings destination")]
    InvalidDestination,

    #[cfg(feature = "v1-parsing")]
    #[display(fmt = "parsing error: json: {}, ini: {}", _0, _1)]
    V1Parsing(serde_json::Error, serde_ini::de::Error),
}

#[derive(Clone, Debug, Deref, DerefMut, PartialEq, Eq)]
pub struct RuntimeSettings {
    #[deref]
    #[deref_mut]
    pub inner: api::RuntimeSettings,

    #[cfg(feature = "v1-parsing")]
    v1_content: Option<String>,
}

impl Default for RuntimeSettings {
    fn default() -> Self {
        RuntimeSettings {
            inner: api::RuntimeSettings {
                polling: api::RuntimePolling {
                    last: DateTime::from_utc(NaiveDateTime::from_timestamp(0, 0), Utc),
                    retries: 0,
                    now: false,
                    server_address: api::ServerAddress::Default,
                },
                update: api::RuntimeUpdate {
                    upgrade_to_installation: None,
                    applied_package_uid: None,
                },
                path: std::path::PathBuf::new(),
                persistent: false,
            },

            #[cfg(feature = "v1-parsing")]
            v1_content: None,
        }
    }
}

impl RuntimeSettings {
    pub fn load(path: &Path) -> Result<Self> {
        let mut this = if path.exists() {
            debug!("loading runtime settings from {:?}", path);
            match fs::read_to_string(path).map_err(Error::from).and_then(|ref s| Self::parse(s)) {
                Ok(v) => v,
                Err(e) => {
                    warn!("failed to load current runtime settings: {}", e);
                    let _ = fs::rename(
                        path,
                        path.with_file_name(format!(
                            "{}.old",
                            path.file_name().unwrap().to_str().unwrap()
                        )),
                    );
                    debug!("using default runtime settings");
                    Self::default()
                }
            }
        } else {
            debug!("runtime settings file {:?} does not exists, using default settings", path);
            Self::default()
        };

        this.path = path.to_path_buf();
        Ok(this)
    }

    fn parse(content: &str) -> Result<Self> {
        let runtime_settings = serde_json::from_str(content).map(|s| RuntimeSettings {
            inner: s,
            #[cfg(feature = "v1-parsing")]
            v1_content: None,
        });

        #[cfg(feature = "v1-parsing")]
        let runtime_settings = runtime_settings.or_else(|e| {
            v1_parse(content, e)
                .map(|s| RuntimeSettings { inner: s, v1_content: Some(content.to_string()) })
        });

        runtime_settings.map_err(Error::from)
    }

    fn save(&self) -> Result<()> {
        if !self.persistent {
            debug!("skipping runtime settings save, using non-persistent");
            return Ok(());
        }

        let parent = self.path.parent().ok_or(Error::InvalidDestination)?;
        if !parent.exists() {
            debug!("creating runtime settings to store state");
            fs::create_dir_all(parent)?;
        }

        fs::write(&self.path, self.serialize()?)?;
        debug!("saved runtime settings to {:?}", &self.path);

        Ok(())
    }

    fn serialize(&self) -> Result<String> {
        Ok(serde_json::to_string(&self.inner)?)
    }

    pub(crate) fn get_inactive_installation_set(&self) -> Result<Set> {
        // If the `upgrade_to_installation` is defined, the current inactive
        // installation_set has already been swapped.
        if let Some(s) = self.update.upgrade_to_installation {
            return Ok(Set(s));
        }

        Ok(installation_set::inactive()?)
    }

    pub(crate) fn enable_persistency(&mut self) {
        self.persistent = true;
    }

    pub(crate) fn is_polling_forced(&self) -> bool {
        self.polling.now
    }

    pub(crate) fn disable_force_poll(&mut self) -> Result<()> {
        debug!("disabling foce poll");
        self.polling.now = false;
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

    pub(crate) fn last_polling(&self) -> DateTime<Utc> {
        self.polling.last
    }

    pub(crate) fn set_last_polling(&mut self, last_polling: DateTime<Utc>) -> Result<()> {
        debug!("updating last polling time");
        self.polling.last = last_polling;
        self.save()
    }

    pub(crate) fn applied_package_uid(&self) -> Option<String> {
        self.update.applied_package_uid.clone()
    }

    pub(crate) fn set_applied_package_uid(&mut self, applied_package_uid: &str) -> Result<()> {
        debug!("marking package {} as installed", applied_package_uid);
        self.update.applied_package_uid = Some(applied_package_uid.to_string());
        self.save()
    }

    pub(crate) fn set_upgrading_to(&mut self, new_install_set: Set) -> Result<()> {
        debug!("setting upgrading to {}", new_install_set);
        self.update.upgrade_to_installation = Some(new_install_set.0);
        self.save()
    }

    pub(crate) fn custom_server_address(&self) -> Option<&str> {
        match &self.polling.server_address {
            api::ServerAddress::Custom(s) => Some(s),
            api::ServerAddress::Default => None,
        }
    }

    pub(crate) fn set_custom_server_address(&mut self, server_address: &str) {
        self.polling.server_address = api::ServerAddress::Custom(server_address.to_owned());
    }

    /// Reset settings that are only need through a single installation
    pub(crate) fn reset_transient_settings(&mut self) {
        // Server address is reset so it doesn't keep probing the last custom server
        // requested
        self.polling.server_address = api::ServerAddress::Default;
    }

    pub(crate) fn reset_installation_settings(&mut self) -> Result<()> {
        debug!("reseting installation settings");
        self.update.upgrade_to_installation = None;
        self.update.applied_package_uid = None;

        // Ensure we do a probe as soon as possible so full update
        // cycle can be finished.
        self.polling.now = true;

        self.save()
    }

    #[cfg(feature = "v1-parsing")]
    pub(crate) fn restore_v1_content(&mut self) -> Result<()> {
        // Restore the original content of the file to not break the rollback
        // procedure when rebooting.
        if let Some(content) = &self.v1_content {
            warn!("restoring previous content of runtime settings for v1 compatibility");
            fs::write(&self.path, content)?;
        }

        Ok(())
    }
}

#[cfg(feature = "v1-parsing")]
fn v1_parse(content: &str, json_err: serde_json::Error) -> Result<api::RuntimeSettings> {
    use crate::utils::deserialize;
    use serde::Deserialize;

    #[derive(Deserialize)]
    #[serde(deny_unknown_fields)]
    #[serde(rename_all = "PascalCase")]
    struct RuntimeSettings {
        polling: RuntimePolling,
        update: RuntimeUpdate,
    }

    #[derive(Deserialize)]
    #[serde(rename_all = "PascalCase")]
    struct RuntimePolling {
        last_poll: DateTime<Utc>,
        retries: usize,
        #[serde(rename = "ProbeASAP")]
        #[serde(deserialize_with = "deserialize::boolean")]
        probe_asap: bool,
    }

    #[derive(Deserialize)]
    #[serde(rename_all = "PascalCase")]
    pub struct RuntimeUpdate {
        pub upgrade_to_installation: i8,
    }

    let old_runtime_settings = serde_ini::de::from_str::<RuntimeSettings>(content)
        .map_err(|ini_err| Error::V1Parsing(json_err, ini_err))?;

    warn!("loaded v1 runtime settings successfully");

    Ok(api::RuntimeSettings {
        polling: api::RuntimePolling {
            last: old_runtime_settings.polling.last_poll,
            retries: old_runtime_settings.polling.retries,
            now: old_runtime_settings.polling.probe_asap,
            server_address: api::ServerAddress::Default,
        },
        update: api::RuntimeUpdate {
            upgrade_to_installation: match old_runtime_settings.update.upgrade_to_installation {
                0 => Some(api::InstallationSet::A),
                1 => Some(api::InstallationSet::B),
                _ => None,
            },
            applied_package_uid: None,
        },
        path: std::path::PathBuf::new(),
        persistent: false,
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use pretty_assertions::assert_eq;

    #[test]
    fn default() {
        let settings = RuntimeSettings::default();
        let expected = RuntimeSettings {
            inner: api::RuntimeSettings {
                polling: api::RuntimePolling {
                    last: DateTime::from_utc(NaiveDateTime::from_timestamp(0, 0), Utc),
                    retries: 0,
                    now: false,
                    server_address: api::ServerAddress::Default,
                },
                update: api::RuntimeUpdate {
                    upgrade_to_installation: None,
                    applied_package_uid: None,
                },
                path: std::path::PathBuf::new(),
                persistent: false,
            },

            #[cfg(feature = "v1-parsing")]
            v1_content: None,
        };

        assert_eq!(Some(settings), Some(expected));
    }

    #[test]
    fn load_and_save() {
        use std::fs;
        use tempfile::NamedTempFile;

        let tempfile = NamedTempFile::new().unwrap();
        let settings_file = tempfile.path();
        fs::remove_file(settings_file).unwrap();

        let mut settings = RuntimeSettings::load(settings_file).unwrap();
        settings.reset_installation_settings().unwrap();

        let new_settings = RuntimeSettings::load(settings_file).unwrap();

        assert_eq!(settings.update, new_settings.update);
    }

    #[test]
    fn load_bad_formated_file() {
        use std::fs;
        use tempfile::NamedTempFile;

        let tempfile = NamedTempFile::new().unwrap();
        let settings_file = tempfile.path();
        fs::write(settings_file, "foo").unwrap();

        let load_result = RuntimeSettings::load(settings_file);
        assert!(load_result.is_ok(), "We should fail when reading a unformated formatted file");

        let old_file = settings_file.with_file_name(format!(
            "{}.old",
            settings_file.file_name().unwrap().to_str().unwrap()
        ));
        let old_content = fs::read_to_string(&old_file).unwrap();
        assert_eq!(
            old_content, "foo",
            "Old file should still be accessible as a .old file in the same directory"
        );
        fs::remove_file(old_file).unwrap();
    }

    #[cfg(feature = "v1-parsing")]
    #[test]
    fn v1_parsing() {
        let sample = r"
[Polling]
LastPoll=2021-06-01T14:38:57-03:00
FirstPoll=2021-05-01T13:33:33-03:00
ExtraInterval=0
Retries=0
ProbeASAP=false

[Update]
UpgradeToInstallation=1
";

        let expected = RuntimeSettings {
            inner: api::RuntimeSettings {
                polling: api::RuntimePolling {
                    last: DateTime::from_utc(
                        DateTime::parse_from_rfc3339("2021-06-01T14:38:57-03:00")
                            .unwrap()
                            .naive_utc(),
                        Utc,
                    ),
                    retries: 0,
                    now: false,
                    server_address: api::ServerAddress::Default,
                },
                update: api::RuntimeUpdate {
                    upgrade_to_installation: Some(api::InstallationSet::B),
                    applied_package_uid: None,
                },
                path: std::path::PathBuf::new(),
                persistent: false,
            },
            v1_content: Some(sample.to_string()),
        };

        assert_eq!(RuntimeSettings::parse(sample).unwrap(), expected);
    }
}
