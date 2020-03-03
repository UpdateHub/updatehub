// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use derive_more::{Display, From};

pub type Result<A> = std::result::Result<A, Error>;

#[derive(Debug, Display, From)]
pub enum Error {
    #[display(fmt = "Agent is busy: {:?}", _0)]
    AgentIsBusy(crate::api::state::Response),
    #[display(fmt = "Abort download was refused: {:?}", _0)]
    AbortDownloadRefused(crate::api::abort_download::Refused),

    #[display(fmt = "Unexpected response: {:?}", _0)]
    UnexpectedResponse(reqwest::Response),
    ClientError(reqwest::Error),
}
