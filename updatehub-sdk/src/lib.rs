// Copyright (C) 2019, 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

//! The `updatehub-sdk` crate is used to communicate with UpdateHub Agent.
//!
//! When running an agent instance, the API provides some methods
//! for communicating with UpdateHub:
//!
//! - [abort_download](Client::abort_download)
//! - [info](Client::info)
//! - [local_install](Client::local_install)
//! - [log](Client::log)
//! - [probe](Client::probe)
//! - [remote_install](Client::remote_install)

pub mod api;
mod client;
mod error;
pub mod listener;
mod serde_helpers;

pub use client::Client;
pub use error::{Error, Result};
