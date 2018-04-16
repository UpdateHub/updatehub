extern crate core;

#[macro_use]
extern crate failure;
#[macro_use]
extern crate failure_derive;

#[macro_use]
extern crate log;

extern crate chrono;
extern crate crypto_hash;
extern crate hex;

extern crate serde;
#[macro_use]
extern crate serde_derive;
extern crate serde_ini;

#[cfg(test)]
#[macro_use]
extern crate serde_json;

#[cfg(not(test))]
extern crate serde_json;

extern crate checked_command;
extern crate cmdline_words_parser;
extern crate parse_duration;

extern crate rand;

#[macro_use]
extern crate hyper;
extern crate reqwest;

extern crate walkdir;

#[cfg(test)]
extern crate mktemp;

#[cfg(test)]
extern crate mockito;

pub mod build_info;

mod serde_helpers;
mod update_package;

pub mod runtime_settings;
pub mod settings;

pub mod client;

pub mod firmware;
pub mod process;

pub mod states;

pub use failure::Error;

use std::result;
pub type Result<T> = result::Result<T, Error>;
