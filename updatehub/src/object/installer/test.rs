// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::Context;
use crate::object::Installer;
use pkg_schema::objects;

impl Installer for objects::Test {
    fn check_requirements(&self) -> super::Result<()> {
        if self.force_check_requirements_fail {
            return Err(std::io::Error::new(
                std::io::ErrorKind::Other,
                "fail to check the requirements",
            )
            .into());
        }
        Ok(())
    }

    fn install(&self, _: &Context) -> super::Result<()> {
        Ok(())
    }
}
