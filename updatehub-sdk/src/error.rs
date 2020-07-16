// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use derive_more::{Display, Error, From};

pub type Result<A> = std::result::Result<A, Error>;

#[derive(Debug, Display, Error, From)]
pub enum Error {
    #[display(fmt = "Agent is busy: {:?}", _0)]
    AgentIsBusy(#[error(not(source))] crate::api::state::Response),

    #[display(fmt = "Abort download was refused: {:?}", _0)]
    AbortDownloadRefused(#[error(not(source))] crate::api::abort_download::Refused),

    #[display(fmt = "Unexpected response: {:?}", _0)]
    UnexpectedResponse(#[error(not(source))] awc::http::StatusCode),

    ConnectError(awc::error::ConnectError),

    SendRequestError(awc::error::SendRequestError),

    PayloadError(awc::error::PayloadError),

    JsonPayloadError(awc::error::JsonPayloadError),
}
