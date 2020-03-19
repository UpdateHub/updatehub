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
            let state = machine.for_current_state(|s| s.name().to_owned());
            if machine.for_current_state(|s| s.can_run_local_install()) {
                crate::logger::start_memory_logging();
                self.stepper.restart(ctx.address());
                self.state.replace(StateMachine::PrepareLocalInstall(State(PrepareLocalInstall {
                    update_file: req.0,
                })));
                return MessageResult(Response::RequestAccepted(state));
            }

            return MessageResult(Response::InvalidState(state));
        }

        unreachable!("Failed to take StateMachine's ownership");
    }
}
