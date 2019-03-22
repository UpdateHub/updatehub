// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::ObjectType;
use serde::Deserialize;

#[derive(Deserialize, PartialEq, Debug)]
#[serde(rename_all = "kebab-case")]
pub(crate) struct Test {
    filename: String,
    sha256sum: String,
    target: String,
    size: u64,
}

impl_object_type!(Test);
