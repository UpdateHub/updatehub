// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::Context;
use crate::object::Installer;
use pkg_schema::objects;

#[async_trait::async_trait]
impl Installer for objects::Test {
    async fn check_requirements(&self, _: &Context) -> super::Result<()> {
        if self.force_check_requirements_fail {
            return Err(std::io::Error::new(
                std::io::ErrorKind::Other,
                "fail to check the requirements",
            )
            .into());
        }
        Ok(())
    }

    async fn install(&self, _: &Context) -> super::Result<()> {
        Ok(())
    }
}
