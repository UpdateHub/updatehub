// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{EntryPoint, State, StateMachine};
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
        if let Some(machine) = &self.state {
            if machine.for_current_state(|s| s.can_run_download_abort()) {
                self.state.replace(StateMachine::EntryPoint(State(EntryPoint {})));
                return MessageResult(Response::RequestAccepted);
            }

            return MessageResult(Response::InvalidState);
        }

        unreachable!("Failed to take StateMachine's ownership");
    }
}
