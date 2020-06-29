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

pub use crate::{build_info::version, settings::Settings, states::run};
use thiserror::Error;

pub type Result<T> = std::result::Result<T, Error>;

#[derive(Debug, Error)]
pub enum Error {
    #[error("Runtime settings error: {0}")]
    RuntimeSettings(#[from] crate::runtime_settings::Error),

    #[error("Settings error: {0}")]
    Settings(#[from] crate::settings::Error),

    #[error("Firmware error: {0}")]
    Firmware(#[from] crate::firmware::Error),

    #[error("Io error: {0}")]
    Io(#[from] std::io::Error),

    #[error("Process error: {0}")]
    Process(#[from] easy_process::Error),
}
