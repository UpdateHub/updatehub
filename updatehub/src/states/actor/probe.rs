// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{Probe, ServerAddress, State, StateMachine};
use actix::{Context, Handler, Message, MessageResult};

pub(crate) struct Request(pub(crate) Option<String>);
pub(crate) enum Response {
    RequestAccepted(String),
    InvalidState(String),
}

impl Message for Request {
    type Result = Response;
}

impl Handler<Request> for super::Machine {
    type Result = MessageResult<Request>;

    fn handle(&mut self, req: Request, _: &mut Context<Self>) -> Self::Result {
        if let Some(machine) = &self.0 {
            let res = machine.for_any_state(|s| s.handle_trigger_probe());
            return match res {
                Response::InvalidState(_) => MessageResult(res),
                Response::RequestAccepted(_) => {
                    self.0.replace(StateMachine::Probe(State(Probe {
                        server_address: req.0.map_or(ServerAddress::Default, ServerAddress::Custom),
                    })));
                    MessageResult(res)
                }
            };
        }

        unreachable!("Failed to take StateMachine's ownership");
    }
}
