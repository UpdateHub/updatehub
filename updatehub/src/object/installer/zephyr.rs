// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::Context;
use crate::object::Installer;
use pkg_schema::objects;
use slog_scope::warn;

impl Installer for objects::Zephyr {
    fn check_requirements(&self, _: &Context) -> super::Result<()> {
        warn!("'zephyr' objects are not supported");
        Err(super::Error::Unsupported)
    }

    fn install(&self, _: &Context) -> super::Result<()> {
        warn!("'zephyr' objects are not supported");
        Err(super::Error::Unsupported)
    }
}
