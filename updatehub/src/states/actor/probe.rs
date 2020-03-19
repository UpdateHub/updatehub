// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{Probe, State, StateMachine};
use actix::{AsyncContext, Context, Handler, Message, MessageResult};

#[derive(Message)]
#[rtype(Response)]
pub(crate) struct Request(pub(crate) Option<String>);

pub(crate) enum Response {
    RequestAccepted(String),
    InvalidState(String),
}

impl Handler<Request> for super::Machine {
    type Result = MessageResult<Request>;

    fn handle(&mut self, req: Request, ctx: &mut Context<Self>) -> Self::Result {
        if let Some(machine) = &self.state {
            let state = machine.for_current_state(|s| s.name().to_owned());
            if machine.for_current_state(|s| s.can_run_trigger_probe()) {
                self.shared_state.runtime_settings.reset_transient_settings();
                if let Some(server_address) = req.0 {
                    self.shared_state.runtime_settings.set_custom_server_address(&server_address);
                }
                self.stepper.restart(ctx.address());
                self.state.replace(StateMachine::Probe(State(Probe {})));
                return MessageResult(Response::RequestAccepted(state));
            }

            return MessageResult(Response::InvalidState(state));
        }

        unreachable!("Failed to take StateMachine's ownership");
    }
}
