// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use thiserror::Error;

pub type Result<A> = std::result::Result<A, Error>;

#[derive(Debug, Error)]
pub enum Error {
    #[error("Agent is busy: {0:?}")]
    AgentIsBusy(crate::api::state::Response),

    #[error("Abort download was refused: {0:?}")]
    AbortDownloadRefused(crate::api::abort_download::Refused),

    #[error("Unexpected response: {0:?}")]
    UnexpectedResponse(reqwest::Response),

    #[error("Client error: {0}")]
    ClientError(#[from] reqwest::Error),
}
