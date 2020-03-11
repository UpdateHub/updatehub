// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::firmware::installation_set::Set;
use chrono::{DateTime, Duration, Utc};
use derive_more::{Deref, DerefMut};
use sdk::api::info::runtime_settings as api;
use slog_scope::debug;
use std::{fs, io, path::Path};
use thiserror::Error;

pub type Result<T> = std::result::Result<T, Error>;

#[derive(Debug, Error)]
pub enum Error {
    #[error("IO error: {0}")]
    Io(#[from] io::Error),

    #[error("Fail with serialization/deserialization: {0}")]
    SerdeJson(#[from] serde_json::Error),

    #[error("Invalid runtime settings destination")]
    InvalidDestination,
}

#[derive(Debug, Deref, DerefMut, PartialEq)]
pub(crate) struct RuntimeSettings(pub(crate) api::RuntimeSettings);

impl Default for RuntimeSettings {
    fn default() -> Self {
        RuntimeSettings(api::RuntimeSettings {
            polling: api::RuntimePolling {
                last: None,
                extra_interval: None,
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
    pub(crate) fn load(path: &Path) -> Result<Self> {
        let mut this = if path.exists() {
            debug!("Loading runtime settings from {:?}...", path);
            Self::parse(&fs::read_to_string(path)?)?
        } else {
            debug!(
                "Runtime settings file {:?} does not exists. Using default runtime settings...",
                path
            );
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
            debug!("Skipping runtime settings save, using non-persistent.");
            return Ok(());
        }

        let parent = self.path.parent().ok_or_else(|| Error::InvalidDestination)?;
        if !parent.exists() {
            debug!("Creating runtime settings to store state.");
            fs::create_dir_all(parent)?;
        }

        debug!("Saving runtime settings from {:?}...", &self.path);
        fs::write(&self.path, self.serialize()?)?;

        Ok(())
    }

    fn serialize(&self) -> Result<String> {
        Ok(serde_json::to_string(&self.0)?)
    }

    pub(crate) fn enable_persistency(&mut self) {
        self.persistent = true;
    }

    pub(crate) fn is_polling_forced(&self) -> bool {
        self.polling.now
    }

    pub(crate) fn force_poll(&mut self) -> Result<()> {
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

    pub(crate) fn set_polling_extra_interval(&mut self, extra_interval: Duration) -> Result<()> {
        self.polling.extra_interval = Some(extra_interval);
        self.save()
    }

    pub(crate) fn last_polling(&self) -> Option<DateTime<Utc>> {
        self.polling.last
    }

    pub(crate) fn set_last_polling(&mut self, last_polling: DateTime<Utc>) -> Result<()> {
        self.polling.last = Some(last_polling);
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
        self.save()
    }
}

#[test]
fn default() {
    use pretty_assertions::assert_eq;
    let settings = RuntimeSettings::default();
    let expected = RuntimeSettings(api::RuntimeSettings {
        polling: api::RuntimePolling {
            last: None,
            extra_interval: None,
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
    settings.force_poll().unwrap();

    let new_settings = RuntimeSettings::load(settings_file).unwrap();

    assert_eq!(settings.update, new_settings.update);
}
