// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
// 

#[derive(Deserialize, PartialEq, Debug)]
#[serde(tag = "mode")]
#[serde(rename_all = "lowercase")]
pub enum Object {
    Test(Test),
}

#[derive(Deserialize, PartialEq, Debug)]
#[serde(rename_all = "kebab-case")]
pub struct Test {
    pub filename: String,
    pub sha256sum: String,
    pub target: String,
}
