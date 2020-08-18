// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

mod copy;
mod flash;
mod imxkobs;
mod mender;
mod raw;
mod raw_delta;
mod tarball;
mod test;
mod ubifs;
mod uboot_env;
mod zephyr;

mod update_package;

/// Internal structures in the Objects for some type validation
pub mod definitions;
/// Objects representing each possible install mode
pub mod objects {
    pub use crate::{
        copy::Copy, flash::Flash, imxkobs::Imxkobs, mender::Mender, raw::Raw, raw_delta::RawDelta,
        tarball::Tarball, test::Test, ubifs::Ubifs, uboot_env::UbootEnv, zephyr::Zephyr,
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
    Mender(Box<objects::Mender>),
    Raw(Box<objects::Raw>),
    #[serde(rename = "raw-delta")]
    RawDelta(Box<objects::RawDelta>),
    Tarball(Box<objects::Tarball>),
    Test(Box<objects::Test>),
    Ubifs(Box<objects::Ubifs>),
    #[serde(rename = "uboot-env")]
    UbootEnv(Box<objects::UbootEnv>),
    Zephyr(Box<objects::Zephyr>),
}
