/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contact@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

use serde_ini;

use std::io;
use std::time::Duration;

use de_helpers::{bool_from_str, duration_from_str, vec_from_str};

const SYSTEM_SETTINGS_PATH: &str = "/etc/updatehub.conf";

#[derive(Default, PartialEq, Deserialize)]
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

    pub fn load(self) -> Result<Self, SettingsError> {
        use std::fs::File;
        use std::io::Read;
        use std::path::Path;

        let path = Path::new(SYSTEM_SETTINGS_PATH);

        if path.exists() {
            info!("Loading system settings from '{}'...",
                  path.to_string_lossy());

            let mut content = String::new();
            File::open(path)?.read_to_string(&mut content)?;

            Ok(self.parse(&content)?)
        } else {
            debug!("System settings file {} does not exists.",
                   path.to_string_lossy());
            info!("Using default system settings...");
            Ok(self)
        }
    }

    fn parse(self, content: &str) -> Result<Self, SettingsError> {
        fn has_protocol_prefix(server: &str) -> bool {
            server.starts_with("http://") || server.starts_with("https://")
        }

        fn has_valid_polling_interval(interval: &Duration) -> bool {
            (*interval >= Duration::new(60, 0))
        }

        let settings = serde_ini::from_str::<Settings>(content)?;

        if !has_valid_polling_interval(&settings.polling.interval) {
            error!("Invalid setting for polling interval. The interval cannot be less than 60 seconds");
            return Err(SettingsError::InvalidInterval);
        }

        if !has_protocol_prefix(&settings.network.server_address) {
            error!("Invalid setting for server address. The server address must use the protocol prefix");
            return Err(SettingsError::InvalidServerAddress);
        }

        Ok(settings)
    }
}

#[derive(Debug)]
pub enum SettingsError {
    Io(io::Error),
    Ini(serde_ini::de::Error),
    InvalidInterval,
    InvalidServerAddress,
}

impl From<io::Error> for SettingsError {
    fn from(err: io::Error) -> SettingsError {
        SettingsError::Io(err)
    }
}

impl From<serde_ini::de::Error> for SettingsError {
    fn from(err: serde_ini::de::Error) -> SettingsError {
        SettingsError::Ini(err)
    }
}

#[derive(Deserialize, PartialEq)]
#[serde(rename_all = "PascalCase")]
pub struct Polling {
    #[serde(deserialize_with = "duration_from_str")] pub interval: Duration,
    #[serde(deserialize_with = "bool_from_str")] pub enabled: bool,
}

impl Default for Polling {
    fn default() -> Self {
        Polling { interval: Duration::new(86_400, 0), // 1 day
                  enabled: true, }
    }
}

#[derive(Deserialize, PartialEq)]
#[serde(rename_all = "PascalCase")]
pub struct Storage {
    #[serde(deserialize_with = "bool_from_str")] pub read_only: bool,
    pub runtime_settings: String,
}

impl Default for Storage {
    fn default() -> Self {
        Storage { read_only: false,
                  runtime_settings: "/var/lib/updatehub.conf".to_string(), }
    }
}

#[derive(Deserialize, PartialEq)]
#[serde(rename_all = "PascalCase")]
pub struct Update {
    pub download_dir: String,
    #[serde(rename = "SupportedInstallModes")]
    #[serde(deserialize_with = "vec_from_str")]
    pub install_modes: Vec<String>,
}

impl Default for Update {
    fn default() -> Self {
        Update { download_dir: "/tmp/updatehub".to_string(),
                 install_modes: ["dry-run", "copy", "flash", "imxkobs", "raw", "tarball", "ubifs"].iter()
                                                                                                  .map(|i| {
                                                                                                           i.to_string()
                                                                                                       })
                                                                                                  .collect(), }
    }
}

#[derive(Deserialize, PartialEq)]
#[serde(rename_all = "PascalCase")]
pub struct Network {
    pub server_address: String,
}

impl Default for Network {
    fn default() -> Self {
        Network { server_address: "https://api.updatehub.io".to_string(), }
    }
}

#[derive(Deserialize, PartialEq)]
#[serde(rename_all = "PascalCase")]
pub struct Firmware {
    pub metadata_path: String,
}

impl Default for Firmware {
    fn default() -> Self {
        Firmware { metadata_path: "/usr/share/updatehub".to_string(), }
    }
}

#[cfg(test)]
mod de_ini {
    use super::*;

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

        let expected =
            Settings { polling: Polling { interval: Duration::new(60, 0),
                                          enabled: false, },
                       storage: Storage { read_only: true,
                                          runtime_settings: "/run/updatehub/state".to_string(), },
                       update: Update { download_dir: "/tmp/download".to_string(),
                                        install_modes: ["mode1", "mode2"].iter().map(|i| i.to_string()).collect(), },
                       network: Network { server_address: "http://localhost".to_string(), },
                       firmware: Firmware { metadata_path: "/tmp/metadata".to_string(), }, };

        assert!(serde_ini::from_str::<Settings>(&ini).map_err(|e| println!("{}", e))
                                                     .ok() == Some(expected));
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
        assert!(Settings::new().parse(&ini).is_err());
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
        assert!(Settings::new().parse(&ini).is_err());
    }

    #[test]
    fn default() {
        let settings = Settings::new();
        let expected = Settings {
            polling: Polling {
                interval: Duration::new(86_400, 0),
                enabled: true,
            },
            storage: Storage {
                read_only: false,
                runtime_settings: "/var/lib/updatehub.conf".to_string(),
            },
            update: Update {
                download_dir: "/tmp/updatehub".to_string(),
                install_modes: ["dry-run", "copy", "flash", "imxkobs", "raw", "tarball", "ubifs"]
                    .iter()
                    .map(|i| i.to_string())
                    .collect(),
            },
            network: Network { server_address: "https://api.updatehub.io".to_string() },
            firmware: Firmware { metadata_path: "/usr/share/updatehub".to_string() },
        };

        assert!(Some(settings) == Some(expected));
    }
}
