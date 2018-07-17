#![cfg_attr(not(feature = "clippy"), allow(unknown_lints))]

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
extern crate hyper;
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

pub mod build_info;
pub mod client;
pub mod firmware;
mod hook;
pub mod runtime_settings;
mod serde_helpers;
pub mod settings;
pub mod states;
mod update_package;
pub use failure::Error;

use std::result;
pub type Result<T> = result::Result<T, Error>;
