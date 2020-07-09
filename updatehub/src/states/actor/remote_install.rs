// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{DirectDownload, State};
use actix::{AsyncContext, Context, Handler, Message, MessageResult};

#[derive(Message)]
#[rtype(Response)]
pub(crate) struct Request(pub(crate) String);

pub(crate) enum Response {
    RequestAccepted(String),
    InvalidState(String),
}

impl Handler<Request> for super::Machine {
    type Result = MessageResult<Request>;

    fn handle(&mut self, req: Request, ctx: &mut Context<Self>) -> Self::Result {
        let machine = self.state.as_ref().expect("Failed to take State's ownership");
        let state = machine.for_current_state(|s| s.name().to_owned());
        if machine.for_current_state(|s| s.is_preemptive_state()) {
            crate::logger::start_memory_logging();
            self.stepper.restart(ctx.address());
            self.state.replace(State::DirectDownload(DirectDownload { url: req.0 }));
            return MessageResult(Response::RequestAccepted(state));
        }

        MessageResult(Response::InvalidState(state))
    }
}
