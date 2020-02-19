// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::{firmware::Metadata, settings::Settings};
use actix::{Context, Handler, Message, MessageResult};
use serde::Serialize;

#[derive(Message)]
#[rtype(Response)]
pub(crate) struct Request;

#[derive(Serialize)]
pub(crate) struct Response {
    #[serde(skip)]
    pub(crate) state: String,
    pub(crate) version: String,
    pub(crate) config: Settings,
    pub(crate) firmware: Metadata,
}

impl Handler<Request> for super::Machine {
    type Result = MessageResult<Request>;

    fn handle(&mut self, _: Request, _: &mut Context<Self>) -> Self::Result {
        if let Some(machine) = &self.state {
            let state = machine.for_any_state(|s| s.name().to_owned());
            return MessageResult(Response {
                state,
                version: crate::version().to_string(),
                config: self.shared_state.settings.clone(),
                firmware: self.shared_state.firmware.clone(),
            });
        }

        unreachable!("Failed to take StateMachine's ownership");
    }
}
