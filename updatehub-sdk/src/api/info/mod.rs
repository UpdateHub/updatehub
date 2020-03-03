// Copyright (C) 2019, 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use serde::{Deserialize, Serialize};

pub mod firmware;
pub mod runtime_settings;
pub mod settings;

#[derive(Clone, Debug, Deserialize, Serialize)]
pub struct Response {
    pub state: String,
    pub version: String,
    pub config: settings::Settings,
    pub firmware: firmware::Metadata,
    pub runtime_settings: runtime_settings::RuntimeSettings,
}
