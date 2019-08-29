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
            let res = machine.for_any_state(|s| s.handle_download_abort());
            return match res {
                Response::InvalidState => MessageResult(res),
                Response::RequestAccepted => {
                    self.state.replace(StateMachine::EntryPoint(State(EntryPoint {})));
                    MessageResult(res)
                }
            };
        }

        unreachable!("Failed to take StateMachine's ownership");
    }
}
