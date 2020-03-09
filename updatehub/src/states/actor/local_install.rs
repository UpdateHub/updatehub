// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{PrepareLocalInstall, State, StateMachine};
use actix::{AsyncContext, Context, Handler, Message, MessageResult};
use std::path::PathBuf;

#[derive(Message)]
#[rtype(Response)]
pub(crate) struct Request(pub(crate) PathBuf);

pub(crate) enum Response {
    RequestAccepted(String),
    InvalidState(String),
}

impl Handler<Request> for super::Machine {
    type Result = MessageResult<Request>;

    fn handle(&mut self, req: Request, ctx: &mut Context<Self>) -> Self::Result {
        if let Some(machine) = &self.state {
            let res = machine.for_any_state(|s| s.handle_local_install());
            return match res {
                Response::InvalidState(_) => MessageResult(res),
                Response::RequestAccepted(_) => {
                    crate::logger::start_memory_logging();
                    self.stepper.restart(ctx.address());
                    self.state.replace(StateMachine::PrepareLocalInstall(State(
                        PrepareLocalInstall { update_file: req.0 },
                    )));
                    MessageResult(res)
                }
            };
        }

        unreachable!("Failed to take StateMachine's ownership");
    }
}
