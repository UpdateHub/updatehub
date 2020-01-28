// Copyright (C) 2018, 2019, 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

#![allow(dead_code)]

mod build_info;
mod client;
mod firmware;
mod http_api;
pub mod logger;
mod mem_drain;
mod object;
mod runtime_settings;
mod serde_helpers;
mod settings;
mod states;
mod update_package;
mod utils;

pub use crate::{build_info::version, settings::Settings, states::run};
use derive_more::{Display, From};

pub type Result<T> = std::result::Result<T, Error>;

#[derive(Debug, Display, From)]
pub enum Error {
    #[display(fmt = "Update package error: {}", _0)]
    UpdatePackage(crate::update_package::Error),
    #[display(fmt = "Runtime settings error: {}", _0)]
    RuntimeSettings(crate::runtime_settings::Error),
    #[display(fmt = "Settings error: {}", _0)]
    Settings(crate::settings::Error),
    #[display(fmt = "Firmware error: {}", _0)]
    Firmware(crate::firmware::Error),
    #[display(fmt = "Client error: {}", _0)]
    Client(crate::client::Error),
    #[display(fmt = "Io error: {}", _0)]
    Io(std::io::Error),
}
