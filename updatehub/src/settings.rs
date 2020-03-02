// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use chrono::Duration;
use derive_more::{Deref, DerefMut, Display, From};
use slog_scope::{debug, error};
use std::{fs, io};

const SYSTEM_SETTINGS_PATH: &str = "/etc/updatehub.conf";

pub use sdk::api::info::settings as api;

pub type Result<T> = std::result::Result<T, Error>;

#[derive(Debug, Display, From)]
pub enum Error {
    #[display(fmt = "IO error: {}", _0)]
    Io(io::Error),
    #[display(fmt = "Fail reading the file: {}", _0)]
    Deserialize(toml::de::Error),
    #[display(fmt = "Fail generating the file: {}", _0)]
    Serialize(toml::ser::Error),
    #[display(fmt = "Invalid interval")]
    InvalidInterval,
    #[display(fmt = "Invalid server address")]
    InvalidServerAddress,
}

#[derive(Clone, Debug, Deref, DerefMut, From, PartialEq)]
pub struct Settings(pub api::Settings);

impl Default for Settings {
    fn default() -> Self {
        Settings(api::Settings {
            polling: api::Polling { interval: Duration::days(1), enabled: true },
            storage: api::Storage {
                read_only: false,
                runtime_settings: "/var/lib/updatehub/runtime_settings.conf".into(),
            },
            update: api::Update {
                download_dir: "/tmp/updatehub".into(),
                supported_install_modes: [
                    "dry-run", "copy", "flash", "imxkobs", "raw", "tarball", "ubifs",
                ]
                .iter()
                .map(|i| (*i).to_string())
                .collect(),
            },
            network: api::Network {
                #[cfg(test)]
                server_address: mockito::server_url().to_string(),
                #[cfg(not(test))]
                server_address: "https://api.updatehub.io".to_string(),
                listen_socket: "localhost:8080".to_string(),
            },
            firmware: api::Firmware { metadata: "/usr/share/updatehub".into() },
        })
    }
}

impl Settings {
    /// Loads the settings from the filesystem. If
    /// `/etc/updatehub.conf` does not exists, it uses the default
    /// settings.
    pub fn load() -> Result<Self> {
        let path = std::path::Path::new(SYSTEM_SETTINGS_PATH);

        if path.exists() {
            debug!("Loading system settings from {:?}...", path);
            Ok(Self::parse(&fs::read_to_string(path)?)?)
        } else {
            debug!(
                "System settings file {:?} does not exists. Using default system settings...",
                path
            );
            Ok(Self::default())
        }
    }

    // This parses the configuration file, taking into account the
    // needed validations for all fields, and returns either `Self` or
    // `Err`.
    fn parse(content: &str) -> Result<Self> {
        let settings = Settings(toml::from_str::<api::Settings>(content)?);

        if settings.polling.interval < Duration::seconds(60) {
            error!(
                "Invalid setting for polling interval. The interval cannot be less than 60 seconds"
            );
            return Err(Error::InvalidInterval);
        }

        if !&settings.network.server_address.starts_with("http://")
            && !&settings.network.server_address.starts_with("https://")
        {
            error!(
                "Invalid setting for server address. The server address must use the protocol prefix"
            );
            return Err(Error::InvalidServerAddress);
        }

        Ok(settings)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn basic_config() {
        let sample = r#"
[network]
server_address="https://api.updatehub.io"
listen_socket="localhost:8080"

[storage]
read_only = false
runtime_settings="/data/updatehub/state.data"

[polling]
enabled=true
interval="60s"

[update]
download_dir="/tmp/updatehub"
supported_install_modes=["copy", "tarball"]

[firmware]
metadata="/usr/share/updatehub"
"#;
        let expected = Settings(api::Settings {
            polling: api::Polling { interval: Duration::minutes(1), enabled: true },
            storage: api::Storage {
                read_only: false,
                runtime_settings: "/data/updatehub/state.data".into(),
            },
            update: api::Update {
                download_dir: "/tmp/updatehub".into(),
                supported_install_modes: ["copy", "tarball"]
                    .iter()
                    .map(|i| (*i).to_string())
                    .collect(),
            },
            network: api::Network {
                server_address: "https://api.updatehub.io".to_string(),
                listen_socket: "localhost:8080".to_string(),
            },
            firmware: api::Firmware { metadata: "/usr/share/updatehub".into() },
        });
        assert_eq!(Settings::parse(sample).unwrap(), expected);
    }

    #[test]
    fn invalid_polling_interval() {
        let sample = r#"
[network]
server_address="https://api.updatehub.io"
listen_socket="localhost:8080"

[storage]
read_only = false
runtime_settings="/data/updatehub/state.data"

[polling]
enabled=true
interval="59s"

[update]
download_dir="/tmp/updatehub"
supported_install_modes=["copy", "tarball"]

[firmware]
metadata="/usr/share/updatehub"
"#;
        assert!(Settings::parse(sample).is_err());
    }

    #[test]
    fn invalid_network_server_address() {
        let sample = r#"
[network]
server_address="api.updatehub.io"
listen_socket="localhost:8080"

[storage]
read_only = false
runtime_settings="/data/updatehub/state.data"

[polling]
enabled=true
interval=60s

[update]
download_dir="/tmp/updatehub"
supported_install_modes=["copy", "tarball"]

[firmware]
metadata="/usr/share/updatehub"
"#;

        assert!(Settings::parse(sample).is_err());
    }

    #[test]
    fn default() {
        use pretty_assertions::assert_eq;
        let mut settings = Settings::default();
        settings.network.server_address = "https://api.updatehub.io".to_string();

        let expected = Settings(api::Settings {
            polling: api::Polling { interval: Duration::days(1), enabled: true },
            storage: api::Storage {
                read_only: false,
                runtime_settings: "/var/lib/updatehub/runtime_settings.conf".into(),
            },
            update: api::Update {
                download_dir: "/tmp/updatehub".into(),
                supported_install_modes: [
                    "dry-run", "copy", "flash", "imxkobs", "raw", "tarball", "ubifs",
                ]
                .iter()
                .map(|i| i.to_string())
                .collect(),
            },
            network: api::Network {
                server_address: "https://api.updatehub.io".to_string(),
                listen_socket: "localhost:8080".to_string(),
            },
            firmware: api::Firmware { metadata: "/usr/share/updatehub".into() },
        });

        assert_eq!(Some(settings), Some(expected));
    }
}
