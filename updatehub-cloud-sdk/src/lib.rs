// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

pub mod api;
mod client;

pub use client::{get, Client};

use derive_more::{Display, Error, From};

pub type Result<T> = std::result::Result<T, Error>;

#[derive(Debug, Display, Error, From)]
pub enum Error {
    #[display(fmt = "Package's signature validation has failed")]
    InvalidSignature,
    #[display(fmt = "Http response is missing Content Length")]
    MissingContentLength,

    Io(std::io::Error),
    JsonParsing(serde_json::Error),
    OpenSsl(openssl::error::ErrorStack),
    ParseInt(std::num::ParseIntError),

    Http(#[error(not(source))] surf::Error),
    #[display(fmt = "Invalid status response: {}", _0)]
    InvalidStatusResponse(#[error(not(source))] surf::StatusCode),
}
