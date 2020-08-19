// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::Context;
use crate::object::Installer;
use pkg_schema::objects;
use slog_scope::warn;

#[async_trait::async_trait(?Send)]
impl Installer for objects::Zephyr {
    async fn check_requirements(&self, _: &Context) -> super::Result<()> {
        warn!("'zephyr' objects are not supported");
        Err(super::Error::Unsupported)
    }

    async fn install(&self, _: &Context) -> super::Result<()> {
        warn!("'zephyr' objects are not supported");
        Err(super::Error::Unsupported)
    }
}
