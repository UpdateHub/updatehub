// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

#[cfg(test)]
mod test;

pub(crate) mod download_abort;
pub(crate) mod info;
pub(crate) mod probe;
mod stepper;

use super::{Idle, Probe, ServerAddress, SharedState, State, StateMachine};
use actix::{Actor, Addr, Arbiter, AsyncContext, Context, Handler, Message, MessageResult};
use slog_scope::info;

pub(crate) struct Machine {
    state: Option<StateMachine>,
    shared_state: SharedState,
    stepper: stepper::Controller,
}

impl Actor for Machine {
    type Context = Context<Self>;

    fn started(&mut self, ctx: &mut Self::Context) {
        info!("Starting State Machine Actor...");
        self.stepper.ensure_running(ctx.address());
    }

    fn stopped(&mut self, _: &mut Self::Context) {
        info!("Stopping State Machine Actor...");
    }
}

impl Machine {
    pub(super) fn start(state: StateMachine, shared_state: SharedState) -> Addr<Self> {
        Machine::start_in_arbiter(&Arbiter::new(), move |_| Machine {
            state: Some(state),
            shared_state,
            stepper: stepper::Controller::default(),
        })
    }
}

struct Step;

enum StepTransition {
    Delayed(std::time::Duration),
    Immediate,
    Never,
}

impl Message for Step {
    type Result = StepTransition;
}

impl Handler<Step> for Machine {
    type Result = MessageResult<Step>;

    fn handle(&mut self, _: Step, _: &mut Context<Self>) -> Self::Result {
        if let Some(machine) = self.state.take() {
            self.state = Some(
                machine
                    .move_to_next_state(&mut self.shared_state)
                    .unwrap_or_else(StateMachine::from),
            );

            return MessageResult(match self.state {
                Some(StateMachine::Park(_)) => StepTransition::Never,
                _ => StepTransition::Immediate,
            });
        }

        unreachable!("Failed to take StateMachine from StateAgent")
    }
}
