// Copyright (C) 2019, 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use std::path::PathBuf;

#[derive(Clone, Debug, Deserialize, PartialEq, Serialize)]
pub struct RuntimeSettings {
    pub polling: RuntimePolling,
    pub update: RuntimeUpdate,
    pub path: PathBuf,
    pub persistent: bool,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Serialize)]
pub struct RuntimePolling {
    pub last: DateTime<Utc>,
    pub retries: usize,
    pub now: bool,
    pub server_address: ServerAddress,
}

#[derive(Clone, Debug, Deserialize, PartialEq, Serialize)]
#[serde(rename_all = "lowercase")]
pub enum ServerAddress {
    Default,
    Custom(String),
}

#[derive(Clone, Debug, Deserialize, PartialEq, Serialize)]
pub struct RuntimeUpdate {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub upgrade_to_installation: Option<InstallationSet>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub applied_package_uid: Option<String>,
}

#[derive(Clone, Copy, Debug, Deserialize, PartialEq, Serialize)]
#[serde(rename_all = "lowercase")]
pub enum InstallationSet {
    A,
    B,
}
