// Copyright (C) 2019, 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use std::path::PathBuf;

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
#[serde(deny_unknown_fields)]
pub struct RuntimeSettings {
    pub polling: RuntimePolling,
    pub update: RuntimeUpdate,
    pub path: PathBuf,
    pub persistent: bool,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
#[serde(deny_unknown_fields)]
pub struct RuntimePolling {
    pub last: DateTime<Utc>,
    pub retries: usize,
    pub now: bool,
    pub server_address: ServerAddress,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
#[serde(rename_all = "lowercase")]
pub enum ServerAddress {
    Default,
    Custom(String),
}

#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
#[serde(deny_unknown_fields)]
pub struct RuntimeUpdate {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub upgrade_to_installation: Option<InstallationSet>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub applied_package_uid: Option<String>,
    /// Present when the pending update wrote the eMMC boot partition of its
    /// installation set, which only then may the eMMC be switched to
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub boot_emmc: Option<RuntimeBootEmmc>,
}

#[derive(Clone, Debug, Default, Deserialize, PartialEq, Eq, Serialize)]
#[serde(deny_unknown_fields)]
pub struct RuntimeBootEmmc {
    /// Kernel boot ID of the boot that installed the pending update, or
    /// rebooted for it, which is only validated from another boot
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub boot_id: Option<String>,
    /// The agent rebooted for the eMMC to boot the firmware of the pending
    /// update, which it only does once
    #[serde(default)]
    pub rebooted: bool,
}

#[derive(Clone, Copy, Debug, Deserialize, PartialEq, Eq, Serialize)]
#[serde(rename_all = "lowercase")]
pub enum InstallationSet {
    A,
    B,
}
