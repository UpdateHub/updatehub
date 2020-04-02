// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

mod copy;
mod flash;
mod imxkobs;
mod mender;
mod raw;
mod tarball;
mod test;
mod ubifs;
mod zephyr;

mod update_package;

/// Internal structures in the Objects for some type validation
pub mod definitions;
/// Objects representing each possible install mode
pub mod objects {
    pub use crate::{
        copy::Copy, flash::Flash, imxkobs::Imxkobs, raw::Raw, tarball::Tarball, test::Test,
        ubifs::Ubifs,
    };
}
pub use update_package::{SupportedHardware, UpdatePackage};

use serde::Deserialize;

/// Represents the install mode for the object data
#[derive(Deserialize, PartialEq, Debug)]
#[serde(tag = "mode")]
#[serde(rename_all = "lowercase")]
pub enum Object {
    Copy(Box<objects::Copy>),
    Flash(Box<objects::Flash>),
    Imxkobs(Box<objects::Imxkobs>),
    Raw(Box<objects::Raw>),
    Tarball(Box<objects::Tarball>),
    Test(Box<objects::Test>),
    Ubifs(Box<objects::Ubifs>),
    // FIXME: Add support for the missing modes: Mende Zephyr
}
