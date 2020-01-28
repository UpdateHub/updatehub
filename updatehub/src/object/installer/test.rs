// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::object::Installer;
use pkg_schema::objects;

impl Installer for objects::Test {
    fn install(&self, _: &std::path::Path) -> super::Result<()> {
        Ok(())
    }
}
