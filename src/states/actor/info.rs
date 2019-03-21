// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::{firmware::Metadata, settings::Settings};
use actix::{Context, Handler, Message, MessageResult};
use serde::Serialize;

pub(crate) struct Request;

#[derive(Serialize)]
pub(crate) struct Payload {
    version: String,
    config: Settings,
    firmware: Metadata,
}

impl Message for Request {
    type Result = Payload;
}

impl Handler<Request> for super::Machine {
    type Result = MessageResult<Request>;

    fn handle(&mut self, _: Request, _: &mut Context<Self>) -> Self::Result {
        MessageResult(Payload {
            version: crate::version().to_string(),
            config: shared_state!().settings.clone(),
            firmware: shared_state!().firmware.clone(),
        })
    }
}
