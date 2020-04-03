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
    #[display("Package's signature validation has failed")]
    InvalidSignature,
    #[display("Http response is missing Content Length")]
    MissingContentLength,

    Io(std::io::Error),
    JsonParsing(serde_json::Error),
    OpenSsl(openssl::error::ErrorStack),
    ParseInt(std::num::ParseIntError),

    #[display("Send Request Error: {}", _0)]
    #[from(ignore)]
    SendRequestError(#[error(not(source))] String),

    InvalidStatusResponse(#[error(not(source))] awc::http::StatusCode),
    Http(awc::error::HttpError),
    ConnectError(awc::error::ConnectError),
    PayloadError(awc::error::PayloadError),
    JsonPayloadError(awc::error::JsonPayloadError),
    InvalidHeader(awc::http::header::InvalidHeaderValue),
    NonStrHeader(awc::http::header::ToStrError),
}
