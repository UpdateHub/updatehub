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
    FirmwareError(firmware::Error),

    #[display("invalid runtime settings destination")]
    InvalidDestination,
}

#[derive(Clone, Debug, Deref, DerefMut, PartialEq)]
pub struct RuntimeSettings(pub api::RuntimeSettings);

impl Default for RuntimeSettings {
    fn default() -> Self {
        RuntimeSettings(api::RuntimeSettings {
            polling: api::RuntimePolling {
                last: DateTime::from_utc(NaiveDateTime::from_timestamp(0, 0), Utc),
                retries: 0,
                now: false,
                server_address: api::ServerAddress::Default,
            },
            update: api::RuntimeUpdate { upgrade_to_installation: None, applied_package_uid: None },
            path: std::path::PathBuf::new(),
            persistent: false,
        })
    }
}

impl RuntimeSettings {
    pub fn load(path: &Path) -> Result<Self> {
        let mut this = if path.exists() {
            debug!("loading runtime settings from {:?}...", path);
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
                    debug!("using default runtime settings...");
                    Self::default()
                }
            }
        } else {
            debug!("runtime settings file {:?} does not exists, using default settings...", path);
            Self::default()
        };

        this.path = path.to_path_buf();
        Ok(this)
    }

    fn parse(content: &str) -> Result<Self> {
        Ok(RuntimeSettings(serde_json::from_str::<api::RuntimeSettings>(&content)?))
    }

    fn save(&self) -> Result<()> {
        if !self.persistent {
            debug!("skipping runtime settings save, using non-persistent.");
            return Ok(());
        }

        let parent = self.path.parent().ok_or_else(|| Error::InvalidDestination)?;
        if !parent.exists() {
            debug!("creating runtime settings to store state.");
            fs::create_dir_all(parent)?;
        }

        debug!("saving runtime settings from {:?}...", &self.path);
        fs::write(&self.path, self.serialize()?)?;

        Ok(())
    }

    fn serialize(&self) -> Result<String> {
        Ok(serde_json::to_string(&self.0)?)
    }

    pub(crate) fn get_inactive_installation_set(&self) -> Result<Set> {
        Ok(match self.update.upgrade_to_installation {
            // If upgrade_to_installation has already been set
            // the current inactive installation_set will already be swapped
            // so we can just install over the same one as before
            Some(s) => Set(s),
            // If no installation has been made so far we can check
            // the system for the current inactive installation set
            None => installation_set::inactive()?,
        })
    }

    pub(crate) fn enable_persistency(&mut self) {
        self.persistent = true;
    }

    pub(crate) fn is_polling_forced(&self) -> bool {
        self.polling.now
    }

    pub(crate) fn disable_force_poll(&mut self) -> Result<()> {
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
        self.polling.last = last_polling;
        self.save()
    }

    pub(crate) fn applied_package_uid(&self) -> Option<String> {
        self.update.applied_package_uid.clone()
    }

    pub(crate) fn set_applied_package_uid(&mut self, applied_package_uid: &str) -> Result<()> {
        self.update.applied_package_uid = Some(applied_package_uid.to_string());
        self.save()
    }

    pub(crate) fn set_upgrading_to(&mut self, new_install_set: Set) -> Result<()> {
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
        self.update.upgrade_to_installation = None;
        self.update.applied_package_uid = None;

        // Ensure we do a probe as soon as possible so full update
        // cycle can be finished.
        self.polling.now = true;

        self.save()
    }
}

#[test]
fn default() {
    use pretty_assertions::assert_eq;
    let settings = RuntimeSettings::default();
    let expected = RuntimeSettings(api::RuntimeSettings {
        polling: api::RuntimePolling {
            last: DateTime::from_utc(NaiveDateTime::from_timestamp(0, 0), Utc),
            retries: 0,
            now: false,
            server_address: api::ServerAddress::Default,
        },
        update: api::RuntimeUpdate { upgrade_to_installation: None, applied_package_uid: None },
        path: std::path::PathBuf::new(),
        persistent: false,
    });

    assert_eq!(Some(settings), Some(expected));
}

#[test]
fn load_and_save() {
    use pretty_assertions::assert_eq;
    use std::fs;
    use tempfile::NamedTempFile;

    let tempfile = NamedTempFile::new().unwrap();
    let settings_file = tempfile.path();
    fs::remove_file(&settings_file).unwrap();

    let mut settings = RuntimeSettings::load(settings_file).unwrap();
    settings.reset_installation_settings().unwrap();

    let new_settings = RuntimeSettings::load(settings_file).unwrap();

    assert_eq!(settings.update, new_settings.update);
}

#[test]
fn load_bad_formated_file() {
    use pretty_assertions::assert_eq;
    use std::fs;
    use tempfile::NamedTempFile;

    let tempfile = NamedTempFile::new().unwrap();
    let settings_file = tempfile.path();
    fs::write(&settings_file, "foo").unwrap();

    let load_result = RuntimeSettings::load(settings_file);
    assert!(load_result.is_ok(), "We should fail when reading a unformated formatted file");

    let old_file = settings_file
        .with_file_name(format!("{}.old", settings_file.file_name().unwrap().to_str().unwrap()));
    let old_content = fs::read_to_string(&old_file).unwrap();
    assert_eq!(
        old_content, "foo",
        "Old file should still be accessible as a .old file in the same directory"
    );
    fs::remove_file(old_file).unwrap();
}
