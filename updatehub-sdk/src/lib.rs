// Copyright (C) 2019, 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

pub mod api;
mod client;
mod error;
pub mod listener;
mod serde_helpers;

pub use client::Client;
pub use error::{Error, Result};
