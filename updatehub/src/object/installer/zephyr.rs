// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::object::Installer;
use pkg_schema::objects;
use slog_scope::warn;

impl Installer for objects::Zephyr {
    fn check_requirements(&self) -> super::Result<()> {
        warn!("'zephyr' objects are not supported");
        Err(super::Error::Unsupported)
    }

    fn install(&self, _: &std::path::Path) -> super::Result<()> {
        warn!("'zephyr' objects are not supported");
        Err(super::Error::Unsupported)
    }
}
