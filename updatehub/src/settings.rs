// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use chrono::Duration;
use derive_more::{Deref, DerefMut, Display, Error, From};
use sdk::api::info::settings as api;
use slog_scope::{debug, error};
use std::{fs, io, path::Path};

pub type Result<T> = std::result::Result<T, Error>;

#[derive(Debug, Display, Error, From)]
pub enum Error {
    Io(io::Error),
    Deserialize(toml::de::Error),
    Serialize(toml::ser::Error),

    #[display("invalid interval")]
    InvalidInterval,
    #[display("invalid server address")]
    InvalidServerAddress,

    #[cfg(feature = "v1-parsing")]
    DeserializeIni(serde_ini::de::Error),
}

#[derive(Clone, Debug, Deref, DerefMut, PartialEq)]
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
    pub fn load(path: &Path) -> Result<Self> {
        if path.exists() {
            debug!("loading system settings from {:?}...", path);
            Ok(Self::parse(&fs::read_to_string(path)?)?)
        } else {
            debug!("system settings file {:?} does not exists, using default settings...", path);
            Ok(Self::default())
        }
    }

    // This parses the configuration file, taking into account the
    // needed validations for all fields, and returns either `Self` or
    // `Err`.
    fn parse(content: &str) -> Result<Self> {
        let res = toml::from_str::<api::Settings>(content);
        let res = res.or_else(|e| v1_parse(content, e.into()));
        let settings = Settings(res?);

        if settings.polling.interval < Duration::seconds(60) {
            error!("invalid setting for polling interval, it cannot be less than 60 seconds");
            return Err(Error::InvalidInterval);
        }

        if !&settings.network.server_address.starts_with("http://")
            && !&settings.network.server_address.starts_with("https://")
        {
            error!("invalid setting for server address, it must use the protocol prefix");
            return Err(Error::InvalidServerAddress);
        }

        Ok(settings)
    }
}

#[cfg(feature = "v1-parsing")]
fn v1_parse(content: &str, _: Error) -> Result<api::Settings> {
    use serde::Deserialize;
    use std::path::PathBuf;

    mod deserialize {
        use chrono::Duration;
        use serde::{de, Deserialize, Deserializer};

        pub(crate) fn duration<'de, D>(deserializer: D) -> Result<Duration, D::Error>
        where
            D: Deserializer<'de>,
        {
            use ms_converter::ms;

            let s = String::deserialize(deserializer)?;
            Ok(Duration::milliseconds(ms(&s).map_err(de::Error::custom)?))
        }

        pub fn boolean<'de, D>(deserializer: D) -> Result<bool, D::Error>
        where
            D: Deserializer<'de>,
        {
            use std::str::FromStr;

            let s = String::deserialize(deserializer)?;
            bool::from_str(&s).map_err(de::Error::custom)
        }

        pub fn string_list<'de, D>(deserializer: D) -> Result<Vec<String>, D::Error>
        where
            D: Deserializer<'de>,
        {
            Ok(String::deserialize(deserializer)?
                .split(',')
                .map(std::string::ToString::to_string)
                .collect())
        }
    }

    #[derive(Deserialize)]
    #[serde(rename_all = "PascalCase")]
    struct Settings {
        #[serde(default)]
        firmware: Firmware,
        network: Network,
        polling: Polling,
        storage: Storage,
        update: Update,
    }

    #[derive(Deserialize)]
    #[serde(rename_all = "PascalCase")]
    struct Polling {
        #[serde(deserialize_with = "deserialize::duration")]
        interval: Duration,
        #[serde(deserialize_with = "deserialize::boolean")]
        enabled: bool,
    }

    #[derive(Deserialize)]
    #[serde(rename_all = "PascalCase")]
    struct Storage {
        #[serde(default)]
        #[serde(deserialize_with = "deserialize::boolean")]
        read_only: bool,
        runtime_settings_path: String,
    }

    #[derive(Deserialize)]
    #[serde(rename_all = "PascalCase")]
    struct Update {
        download_dir: PathBuf,
        #[serde(deserialize_with = "deserialize::string_list")]
        supported_install_modes: Vec<String>,
    }

    #[derive(Deserialize)]
    #[serde(rename_all = "PascalCase")]
    struct Network {
        server_address: String,
        #[serde(default)]
        listen_socket: String,
    }

    #[derive(Deserialize)]
    #[serde(rename_all = "PascalCase")]
    #[serde(default)]
    struct Firmware {
        metadata_path: PathBuf,
    }

    impl Default for Polling {
        fn default() -> Self {
            Self { interval: Duration::days(1), enabled: true }
        }
    }

    impl Default for Storage {
        fn default() -> Self {
            Self {
                read_only: false,
                runtime_settings_path: "/var/lib/updatehub/runtime_settings.conf".into(),
            }
        }
    }

    impl Default for Update {
        fn default() -> Self {
            Self {
                download_dir: "/tmp/updatehub".into(),
                supported_install_modes: [
                    "dry-run", "copy", "flash", "imxkobs", "raw", "tarball", "ubifs",
                ]
                .iter()
                .map(|i| i.to_string())
                .collect(),
            }
        }
    }

    impl Default for Network {
        fn default() -> Self {
            Self {
                server_address: "https://api.updatehub.io".to_string(),
                listen_socket: "localhost:8080".to_string(),
            }
        }
    }

    impl Default for Firmware {
        fn default() -> Self {
            Self { metadata_path: "/usr/share/updatehub".into() }
        }
    }

    let old_settings = serde_ini::de::from_str::<Settings>(content)?;

    Ok(api::Settings {
        firmware: api::Firmware { metadata: old_settings.firmware.metadata_path },
        network: api::Network {
            server_address: old_settings.network.server_address,
            listen_socket: old_settings.network.listen_socket,
        },
        polling: api::Polling {
            interval: old_settings.polling.interval,
            enabled: old_settings.polling.enabled,
        },
        storage: api::Storage {
            read_only: old_settings.storage.read_only,
            runtime_settings: old_settings.storage.runtime_settings_path.into(),
        },
        update: api::Update {
            download_dir: old_settings.update.download_dir,
            supported_install_modes: old_settings.update.supported_install_modes,
        },
    })
}

#[cfg(not(feature = "v1-parsing"))]
#[inline]
fn v1_parse(_: &str, e: Error) -> Result<api::Settings> {
    Err(e)
}

#[cfg(test)]
mod tests {
    use super::*;
    use pretty_assertions::assert_eq;

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

    #[cfg(feature = "v1-parsing")]
    #[test]
    fn v1_parsing() {
        let sample = r"
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

        let expected = Settings(api::Settings {
            polling: api::Polling { interval: Duration::minutes(1), enabled: false },
            storage: api::Storage {
                read_only: false,
                runtime_settings: "/run/updatehub/state".into(),
            },
            update: api::Update {
                download_dir: "/tmp/download".into(),
                supported_install_modes: ["mode1", "mode2"].iter().map(|i| i.to_string()).collect(),
            },
            network: api::Network {
                server_address: "http://localhost".to_string(),
                listen_socket: "localhost:8313".to_string(),
            },
            firmware: api::Firmware { metadata: "/usr/share/updatehub".into() },
        });

        assert_eq!(Settings::parse(sample).unwrap(), expected);
    }
}
