// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

#![cfg_attr(not(feature = "clippy"), allow(unknown_lints))]
#![allow(dead_code)]

extern crate chrono;
extern crate core;
extern crate crypto_hash;
extern crate easy_process;
extern crate hex;
extern crate parse_duration;
extern crate rand;
extern crate reqwest;
extern crate serde;
extern crate serde_ini;
extern crate walkdir;

#[macro_use]
extern crate failure;
#[macro_use]
extern crate failure_derive;

#[macro_use]
extern crate log;

#[macro_use]
extern crate serde_derive;
#[cfg(not(test))]
extern crate serde_json;

#[cfg(test)]
extern crate mockito;
#[cfg(test)]
extern crate tempfile;
#[cfg(test)]
#[macro_use]
extern crate serde_json;

mod build_info;
mod client;
mod firmware;
mod runtime_settings;
mod serde_helpers;
mod settings;
mod states;
mod update_package;

use std::result;
pub type Result<T> = result::Result<T, failure::Error>;

pub use settings::Settings;

pub use build_info::version;
pub use states::run;
