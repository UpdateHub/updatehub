// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{EntryPoint, StateMachine};
use actix::{Context, Handler, Message, MessageResult};

#[derive(Message)]
#[rtype(Response)]
pub struct Request;

pub enum Response {
    RequestAccepted,
    InvalidState,
}

impl Handler<Request> for super::Machine {
    type Result = MessageResult<Request>;

    fn handle(&mut self, _: Request, _: &mut Context<Self>) -> Self::Result {
        let machine = self.state.as_ref().expect("Failed to take StateMachine's ownership");

        if machine.for_current_state(|s| s.is_handling_download()) {
            self.state.replace(StateMachine::EntryPoint(EntryPoint {}));
            return MessageResult(Response::RequestAccepted);
        }

        MessageResult(Response::InvalidState)
    }
}
