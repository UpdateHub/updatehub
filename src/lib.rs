// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

#![allow(dead_code)]

mod build_info;
mod client;
mod firmware;
mod runtime_settings;
mod serde_helpers;
mod settings;
mod states;
mod update_package;

pub type Result<T> = std::result::Result<T, failure::Error>;
pub use crate::{build_info::version, settings::Settings, states::run};
