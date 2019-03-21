// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{Idle, State, StateChangeImpl, StateMachine};
use actix::{Context, Handler, Message, MessageResult};

pub struct Request;
pub enum Response {
    RequestAccepted,
    InvalidState,
}

impl Message for Request {
    type Result = Response;
}

impl Handler<Request> for super::Machine {
    type Result = MessageResult<Request>;

    fn handle(&mut self, _: Request, _: &mut Context<Self>) -> Self::Result {
        if let Some(machine) = self.0.take() {
            return for_any_state!(machine, s, {
                match s.handle_download_abort() {
                    r @ Response::InvalidState => MessageResult(r),
                    r @ Response::RequestAccepted => {
                        self.0 = Some(StateMachine::Idle(State(Idle {})));
                        MessageResult(r)
                    }
                }
            });
        }

        unreachable!("Failed to take StateMachine's ownership");
    }
}
