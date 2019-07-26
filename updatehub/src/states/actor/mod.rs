// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

#[cfg(test)]
mod test;

pub(crate) mod download_abort;
pub(crate) mod info;
pub(crate) mod probe;

use super::{Idle, Probe, ServerAddress, SharedState, State, StateMachine};
use actix::{Actor, Context, Handler, Message, MessageResult};
use slog::slog_error;
use slog_scope::error;

pub struct Machine {
    state: Option<StateMachine>,
    shared_state: SharedState,
}

impl Actor for Machine {
    type Context = Context<Self>;
}

impl Machine {
    pub(super) fn new(state: StateMachine, shared_state: SharedState) -> Self {
        Machine {
            state: Some(state),
            shared_state,
        }
    }
}

pub struct Step;

impl Message for Step {
    type Result = ();
}

impl Handler<Step> for Machine {
    type Result = MessageResult<Step>;

    fn handle(&mut self, _req: Step, _ctx: &mut Context<Self>) -> Self::Result {
        if let Some(machine) = self.state.take() {
            self.state = Some(
                machine
                    .move_to_next_state(&mut self.shared_state)
                    .unwrap_or_else(|e| {
                        error!("Error: {}. Moving to Idle state.", e);
                        StateMachine::Idle(State(Idle {}))
                    }),
            );

            return MessageResult(());
        }

        unreachable!("Failed to take StateMachine from StateAgent")
    }
}
