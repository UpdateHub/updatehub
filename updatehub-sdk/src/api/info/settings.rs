// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::serde_helpers;
use chrono::Duration;
use serde::{Deserialize, Serialize};
use std::path::PathBuf;

#[derive(Clone, Debug, Deserialize, PartialEq, Serialize)]
#[serde(deny_unknown_fields)]
pub struct Settings {
    pub firmware: Firmware,
    pub network: Network,
    pub polling: Polling,
    pub storage: Storage,
    pub update: Update,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Serialize)]
#[serde(deny_unknown_fields)]
pub struct Firmware {
    pub metadata: PathBuf,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Serialize)]
#[serde(deny_unknown_fields)]
pub struct Network {
    pub server_address: String,
    pub listen_socket: String,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Serialize)]
#[serde(deny_unknown_fields)]
pub struct Polling {
    #[serde(with = "serde_helpers::duration")]
    pub interval: Duration,
    pub enabled: bool,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Serialize)]
#[serde(deny_unknown_fields)]
pub struct Storage {
    /// Determine if it should run on read-only mode or not. By
    /// default, read-only mode is disabled.
    pub read_only: bool,
    /// Define where the runtime settings are stored. By default,
    /// those are stored in
    /// `/var/lib/updatehub/runtime_settings.conf`.
    pub runtime_settings: PathBuf,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Serialize)]
#[serde(deny_unknown_fields)]
pub struct Update {
    pub download_dir: PathBuf,
    pub supported_install_modes: Vec<String>,
}
