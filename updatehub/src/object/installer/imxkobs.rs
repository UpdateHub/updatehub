// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::object::Installer;
use pkg_schema::objects;

impl Installer for objects::Imxkobs {
    fn check_requirements(&self) -> Result<(), failure::Error> {
        unimplemented!("FIXME: implement check_requirements for Flash object")
    }

    #[allow(unused_variables)]
    fn install(&self, download_dir: &std::path::Path) -> Result<(), failure::Error> {
        unimplemented!("FIXME: implement install for Flash object")
    }
}
