// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
//

use Result;

use chrono::Duration;
use serde_ini;

use std::io;
use std::path::PathBuf;

use serde_helpers::de;

const SYSTEM_SETTINGS_PATH: &str = "/etc/updatehub.conf";

#[cfg(not(test))]
const SERVER_URL: &str = "https://api.updatehub.io";

// When running inside a test environment we default to the mock
// server
#[cfg(test)]
use mockito;
#[cfg(test)]
const SERVER_URL: &str = mockito::SERVER_URL;

#[derive(Debug, Default, PartialEq, Deserialize)]
#[serde(rename_all = "PascalCase")]
pub struct Settings {
    pub polling: Polling,
    pub storage: Storage,
    pub update: Update,
    pub network: Network,
    pub firmware: Firmware,
}

impl Settings {
    pub fn new() -> Self {
        Settings::default()
    }

    pub fn load(self) -> Result<Self> {
        use std::fs::File;
        use std::io::Read;
        use std::path::Path;

        let path = Path::new(SYSTEM_SETTINGS_PATH);

        if path.exists() {
            info!(
                "Loading system settings from '{}'...",
                path.to_string_lossy()
            );

            let mut content = String::new();
            File::open(path)?.read_to_string(&mut content)?;

            Ok(Settings::parse(&content)?)
        } else {
            debug!(
                "System settings file {} does not exists.",
                path.to_string_lossy()
            );
            info!("Using default system settings...");
            Ok(self)
        }
    }

    fn parse(content: &str) -> Result<Self> {
        let settings = serde_ini::from_str::<Settings>(content)?;

        if settings.polling.interval < Duration::seconds(60) {
            error!(
                "Invalid setting for polling interval. The interval cannot be less than 60 seconds"
            );
            return Err(SettingsError::InvalidInterval.into());
        }

        if !&settings.network.server_address.starts_with("http://")
            && !&settings.network.server_address.starts_with("https://")
        {
            error!("Invalid setting for server address. The server address must use the protocol prefix");
            return Err(SettingsError::InvalidServerAddress.into());
        }

        Ok(settings)
    }
}

#[derive(Debug, Fail)]
pub enum SettingsError {
    #[cause]
    #[fail(display = "IO error")]
    Io(io::Error),
    #[cause]
    #[fail(display = "Invalid INI fail")]
    Ini(serde_ini::de::Error),
    #[fail(display = "Invalid interval")]
    InvalidInterval,
    #[fail(display = "Invalid server address")]
    InvalidServerAddress,
}

#[derive(Debug, Deserialize, PartialEq)]
#[serde(rename_all = "PascalCase")]
pub struct Polling {
    #[serde(deserialize_with = "de::duration_from_str")]
    pub interval: Duration,
    #[serde(deserialize_with = "de::bool_from_str")]
    pub enabled: bool,
}

impl Default for Polling {
    fn default() -> Self {
        Polling {
            interval: Duration::days(1),
            enabled: true,
        }
    }
}

#[derive(Debug, Deserialize, PartialEq)]
#[serde(rename_all = "PascalCase")]
pub struct Storage {
    #[serde(deserialize_with = "de::bool_from_str")]
    pub read_only: bool,
    pub runtime_settings: String,
}

impl Default for Storage {
    fn default() -> Self {
        Storage {
            read_only: false,
            runtime_settings: "/var/lib/updatehub.conf".into(),
        }
    }
}

#[derive(Debug, Deserialize, PartialEq)]
#[serde(rename_all = "PascalCase")]
pub struct Update {
    pub download_dir: PathBuf,
    #[serde(rename = "SupportedInstallModes")]
    #[serde(deserialize_with = "de::vec_from_str")]
    pub install_modes: Vec<String>,
}

impl Default for Update {
    fn default() -> Self {
        Update {
            download_dir: "/tmp/updatehub".into(),
            install_modes: [
                "dry-run", "copy", "flash", "imxkobs", "raw", "tarball", "ubifs",
            ]
                .iter()
                .map(|i| i.to_string())
                .collect(),
        }
    }
}

#[derive(Debug, Deserialize, PartialEq)]
#[serde(rename_all = "PascalCase")]
pub struct Network {
    pub server_address: String,
}

impl Default for Network {
    fn default() -> Self {
        Network {
            server_address: SERVER_URL.into(),
        }
    }
}

#[derive(Debug, Deserialize, PartialEq)]
#[serde(rename_all = "PascalCase")]
pub struct Firmware {
    pub metadata_path: PathBuf,
}

impl Default for Firmware {
    fn default() -> Self {
        Firmware {
            metadata_path: "/usr/share/updatehub".into(),
        }
    }
}

#[test]
fn ok() {
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
ServerAddress=http://localhost

[Firmware]
MetadataPath=/tmp/metadata
";

    let expected = Settings {
        polling: Polling {
            interval: Duration::seconds(60),
            enabled: false,
        },
        storage: Storage {
            read_only: true,
            runtime_settings: "/run/updatehub/state".into(),
        },
        update: Update {
            download_dir: "/tmp/download".into(),
            install_modes: ["mode1", "mode2"].iter().map(|i| i.to_string()).collect(),
        },
        network: Network {
            server_address: "http://localhost".into(),
        },
        firmware: Firmware {
            metadata_path: "/tmp/metadata".into(),
        },
    };

    assert_eq!(
        serde_ini::from_str::<Settings>(ini)
            .map_err(|e| println!("{}", e))
            .ok(),
        Some(expected)
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
    let settings = Settings::new();
    let expected = Settings {
        polling: Polling {
            interval: Duration::days(1),
            enabled: true,
        },
        storage: Storage {
            read_only: false,
            runtime_settings: "/var/lib/updatehub.conf".into(),
        },
        update: Update {
            download_dir: "/tmp/updatehub".into(),
            install_modes: [
                "dry-run", "copy", "flash", "imxkobs", "raw", "tarball", "ubifs",
            ]
                .iter()
                .map(|i| i.to_string())
                .collect(),
        },
        network: Network {
            server_address: SERVER_URL.into(),
        },
        firmware: Firmware {
            metadata_path: "/usr/share/updatehub".into(),
        },
    };

    assert_eq!(Some(settings), Some(expected));
}
