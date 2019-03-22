// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{Probe, ServerAddress, State, StateChangeImpl, StateMachine};
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
        if let Some(machine) = self.0.take() {
            return for_any_state!(machine, s, {
                match s.handle_trigger_probe() {
                    r @ Response::InvalidState(_) => MessageResult(r),
                    r @ Response::RequestAccepted(_) => {
                        self.0 = Some(StateMachine::Probe(State(Probe {
                            server_address: req
                                .0
                                .map_or(ServerAddress::Default, ServerAddress::Custom),
                        })));
                        MessageResult(r)
                    }
                }
            });
        }

        unreachable!("Failed to take StateMachine's ownership");
    }
}
