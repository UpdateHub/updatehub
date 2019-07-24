// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

#[cfg(test)]
mod test;

pub(crate) mod download_abort;
pub(crate) mod info;
pub(crate) mod probe;

use super::{Idle, Probe, ServerAddress, State, StateMachine};
use actix::{Actor, Context, Handler, Message, MessageResult};
use slog::slog_error;
use slog_scope::error;

pub struct Machine(Option<StateMachine>);

impl Actor for Machine {
    type Context = Context<Self>;
}

impl Machine {
    pub(super) fn new(machine: StateMachine) -> Self {
        Machine(Some(machine))
    }
}

pub struct Step;

impl Message for Step {
    type Result = ();
}

impl Handler<Step> for Machine {
    type Result = MessageResult<Step>;

    fn handle(&mut self, _req: Step, _ctx: &mut Context<Self>) -> Self::Result {
        if let Some(machine) = self.0.take() {
            self.0 = Some(machine.move_to_next_state().unwrap_or_else(|e| {
                error!("Error: {}. Moving to Idle state.", e);
                StateMachine::Idle(State(Idle {}))
            }));

            return MessageResult(());
        }

        unreachable!("Failed to take StateMachine from StateAgent")
    }
}
