// Copyright (C) 2018, 2019, 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

mod build_info;
mod firmware;
mod http_api;
pub mod logger;
mod mem_drain;
mod object;
mod runtime_settings;
mod settings;
mod states;
mod update_package;
mod utils;

#[cfg(test)]
mod cloud_mock;

#[cfg(feature = "test-env")]
pub mod tests;

#[cfg(test)]
pub(crate) use crate::cloud_mock::Client as CloudClient;
#[cfg(not(test))]
pub(crate) use cloud::Client as CloudClient;

pub use crate::{build_info::version, states::run};
use derive_more::{Display, Error, From};

pub type Result<T> = std::result::Result<T, Error>;

#[derive(Debug, Display, Error, From)]
pub enum Error {
    RuntimeSettings(crate::runtime_settings::Error),
    Settings(crate::settings::Error),
    Firmware(crate::firmware::Error),
    Io(std::io::Error),
    Process(easy_process::Error),
}
