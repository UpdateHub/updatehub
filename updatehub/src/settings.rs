// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::serde_helpers::{de, ser};

use chrono::Duration;
use derive_more::{Display, From};
use serde::{Deserialize, Serialize};
use serde_ini;
use slog_scope::{debug, error};
use std::{io, path::PathBuf};

const SYSTEM_SETTINGS_PATH: &str = "/etc/updatehub.conf";

// When running inside a test environment we default to the mock
// server
#[cfg(test)]
use mockito;

#[derive(Debug, Default, PartialEq, Deserialize, Serialize, Clone)]
#[serde(rename_all = "PascalCase")]
pub struct Settings {
    #[serde(default)]
    pub(crate) firmware: Firmware,
    pub(crate) network: Network,
    pub(crate) polling: Polling,
    pub(crate) storage: Storage,
    pub(crate) update: Update,
}

pub type Result<T> = std::result::Result<T, Error>;

#[derive(Debug, Display, From)]
pub enum Error {
    #[display(fmt = "IO error: {}", _0)]
    Io(io::Error),
    #[display(fmt = "Invalid INI fail: {}", _0)]
    Ini(serde_ini::de::Error),
    #[display(fmt = "Invalid interval")]
    InvalidInterval,
    #[display(fmt = "Invalid server address")]
    InvalidServerAddress,
}

impl Settings {
    /// Loads the settings from the filesystem. If
    /// `/etc/updatehub.conf` does not exists, it uses the default
    /// settings.
    pub fn load() -> Result<Self> {
        use std::{fs::File, io::Read, path::Path};

        let path = Path::new(SYSTEM_SETTINGS_PATH);

        if path.exists() {
            debug!("Loading system settings from '{}'...", path.to_string_lossy());

            let mut content = String::new();
            File::open(path)?.read_to_string(&mut content)?;

            Ok(Self::parse(&content)?)
        } else {
            debug!(
                "System settings file {} does not exists. Using default system settings...",
                path.to_string_lossy()
            );
            Ok(Self::default())
        }
    }

    // This parses the configuration file, taking into account the
    // needed validations for all fields, and returns either `Self` or
    // `Err`.
    fn parse(content: &str) -> Result<Self> {
        let settings = serde_ini::from_str::<Self>(content)?;

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

#[derive(Debug, Deserialize, PartialEq, Serialize, Clone)]
#[serde(rename_all = "PascalCase")]
pub struct Polling {
    #[serde(deserialize_with = "de::duration_from_str", serialize_with = "ser::duration_to_int")]
    /// Inverval to automatically poll the server for update. By
    /// default, it uses 1 day of interval.
    pub interval: Duration,
    #[serde(deserialize_with = "de::bool_from_str")]
    /// Defines if automatic polling is enabled or not. By default it
    /// is enabled.
    pub enabled: bool,
}

impl Default for Polling {
    fn default() -> Self {
        Self { interval: Duration::days(1), enabled: true }
    }
}

#[derive(Debug, Deserialize, PartialEq, Serialize, Clone)]
#[serde(rename_all = "PascalCase")]
pub struct Storage {
    /// Determine if it should run on read-only mode or not. By
    /// default, read-only mode is disabled.
    #[serde(deserialize_with = "de::bool_from_str")]
    #[serde(default)]
    pub read_only: bool,
    /// Define where the runtime settings are stored. By default,
    /// those are stored in
    /// `/var/lib/updatehub/runtime_settings.conf`.
    #[serde(rename = "RuntimeSettingsPath")]
    pub runtime_settings: String,
}

impl Default for Storage {
    fn default() -> Self {
        Self {
            read_only: false,
            runtime_settings: "/var/lib/updatehub/runtime_settings.conf".into(),
        }
    }
}

#[derive(Debug, Deserialize, PartialEq, Serialize, Clone)]
#[serde(rename_all = "PascalCase")]
pub struct Update {
    pub download_dir: PathBuf,
    #[serde(rename = "SupportedInstallModes")]
    #[serde(deserialize_with = "de::vec_from_str")]
    pub install_modes: Vec<String>,
}

impl Default for Update {
    fn default() -> Self {
        Self {
            download_dir: "/tmp/updatehub".into(),
            install_modes: ["dry-run", "copy", "flash", "imxkobs", "raw", "tarball", "ubifs"]
                .iter()
                .map(|i| (*i).to_string())
                .collect(),
        }
    }
}

#[derive(Debug, Deserialize, PartialEq, Serialize, Clone)]
#[serde(rename_all = "PascalCase")]
pub struct Network {
    pub server_address: String,
    #[serde(default = "default_listen_socket")]
    pub listen_socket: String,
}

fn default_listen_socket() -> String {
    "localhost:8080".to_string()
}

impl Default for Network {
    fn default() -> Self {
        #[cfg(test)]
        let server_address = mockito::server_url().to_string();
        #[cfg(not(test))]
        let server_address = "https://api.updatehub.io".to_string();

        Self { server_address, listen_socket: default_listen_socket() }
    }
}

#[derive(Debug, Deserialize, PartialEq, Serialize, Clone)]
#[serde(rename_all = "PascalCase")]
#[serde(default)]
pub struct Firmware {
    pub metadata_path: PathBuf,
}

impl Default for Firmware {
    fn default() -> Self {
        Self { metadata_path: "/usr/share/updatehub".into() }
    }
}

#[cfg(test)]
mod test {
    use super::*;

    #[test]
    fn ok() {
        use pretty_assertions::assert_eq;
        let ini = r"
[Polling]
Interval=60s
Enabled=false

[Storage]
RuntimeSettingsPath=/run/updatehub/state

[Update]
DownloadDir=/tmp/download
SupportedInstallModes=mode1,mode2

[Network]
ServerAddress=http://localhost
";

        let expected = Settings {
            polling: Polling { interval: Duration::seconds(60), enabled: false },
            storage: Storage { read_only: false, runtime_settings: "/run/updatehub/state".into() },
            update: Update {
                download_dir: "/tmp/download".into(),
                install_modes: ["mode1", "mode2"].iter().map(|i| i.to_string()).collect(),
            },
            network: Network {
                server_address: "http://localhost".into(),
                listen_socket: "localhost:8080".into(),
            },
            firmware: Firmware { metadata_path: "/usr/share/updatehub".into() },
        };

        assert_eq!(
            serde_ini::from_str::<Settings>(ini).map_err(|e| println!("{}", e)).unwrap(),
            expected
        );
    }

    #[test]
    fn custom_listen_socket() {
        use pretty_assertions::assert_eq;
        let ini = r"
[Polling]
Interval=60s
Enabled=false

[Storage]
RuntimeSettingsPath=/run/updatehub/state

[Update]
DownloadDir=/tmp/download
SupportedInstallModes=mode1,mode2

[Network]
ServerAddress=http://localhost
ListenSocket=localhost:8313
";

        let expected = Settings {
            polling: Polling { interval: Duration::seconds(60), enabled: false },
            storage: Storage { read_only: false, runtime_settings: "/run/updatehub/state".into() },
            update: Update {
                download_dir: "/tmp/download".into(),
                install_modes: ["mode1", "mode2"].iter().map(|i| i.to_string()).collect(),
            },
            network: Network {
                server_address: "http://localhost".into(),
                listen_socket: "localhost:8313".into(),
            },
            firmware: Firmware { metadata_path: "/usr/share/updatehub".into() },
        };

        assert_eq!(
            serde_ini::from_str::<Settings>(ini).map_err(|e| println!("{}", e)).unwrap(),
            expected
        );
    }

    #[test]
    fn invalid_polling_interval() {
        let ini = r"
[Polling]
Interval=59s
Enabled=false

[Storage]
ReadOnly=true
RuntimeSettings=/run/updatehub/state

[Update]
DownloadDir=/tmp/download
SupportedInstallModes=mode1,mode2

[Network]
ServerAddress=http://localhost

[Firmware]
MetadataPath=/tmp/metadata
";
        assert!(Settings::parse(ini).is_err());
    }

    #[test]
    fn invalid_network_server_address() {
        let ini = r"
[Polling]
Interval=60s
Enabled=false

[Storage]
ReadOnly=true
RuntimeSettings=/run/updatehub/state

[Update]
DownloadDir=/tmp/download
SupportedInstallModes=mode1,mode2

[Network]
ServerAddress=localhost

[Firmware]
MetadataPath=/tmp/metadata
";

        assert!(Settings::parse(ini).is_err());
    }

    #[test]
    fn default() {
        use pretty_assertions::assert_eq;
        let mut settings = Settings::default();
        settings.network.server_address = "https://api.updatehub.io".to_string();

        let expected = Settings {
            polling: Polling { interval: Duration::days(1), enabled: true },
            storage: Storage {
                read_only: false,
                runtime_settings: "/var/lib/updatehub/runtime_settings.conf".into(),
            },
            update: Update {
                download_dir: "/tmp/updatehub".into(),
                install_modes: ["dry-run", "copy", "flash", "imxkobs", "raw", "tarball", "ubifs"]
                    .iter()
                    .map(|i| i.to_string())
                    .collect(),
            },
            network: Network {
                server_address: "https://api.updatehub.io".to_string(),
                listen_socket: "localhost:8080".to_string(),
            },
            firmware: Firmware { metadata_path: "/usr/share/updatehub".into() },
        };

        assert_eq!(Some(settings), Some(expected));
    }
}
