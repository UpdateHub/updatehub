// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

#[macro_use]
mod macros;

pub(crate) mod info;
pub(crate) mod installer;

pub(crate) use self::{info::Info, installer::Installer};
