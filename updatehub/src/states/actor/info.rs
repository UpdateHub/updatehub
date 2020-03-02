// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use actix::{Context, Handler, Message, MessageResult};

#[derive(Message)]
#[rtype(Response)]
pub(crate) struct Request;

pub(crate) use sdk::api::info::Response;

impl Handler<Request> for super::Machine {
    type Result = MessageResult<Request>;

    fn handle(&mut self, _: Request, _: &mut Context<Self>) -> Self::Result {
        if let Some(machine) = &self.state {
            let state = machine.for_any_state(|s| s.name().to_owned());
            return MessageResult(Response {
                state,
                version: crate::version().to_string(),
                config: self.shared_state.settings.0.clone(),
                firmware: self.shared_state.firmware.0.clone(),
                runtime_settings: self.shared_state.runtime_settings.clone(),
            });
        }

        unreachable!("Failed to take StateMachine's ownership");
    }
}
