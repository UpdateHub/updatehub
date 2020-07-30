// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use serde::Deserialize;

#[derive(Deserialize, PartialEq, Debug)]
#[serde(rename_all = "kebab-case")]
pub struct Test {
    pub filename: String,
    pub sha256sum: String,
    pub target: String,
    pub size: u64,
    pub force_check_requirements_fail: bool,
}
