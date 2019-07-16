// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{ObjectInstaller, ObjectType};
use serde::Deserialize;

#[derive(Deserialize, PartialEq, Debug)]
#[serde(rename_all = "kebab-case")]
pub(crate) struct Test {
    filename: String,
    sha256sum: String,
    target: String,
    size: u64,
}

impl ObjectInstaller for Test {
    fn install(&self, _: std::path::PathBuf) -> Result<(), failure::Error> {
        Ok(())
    }
}

impl_object_type!(Test);
