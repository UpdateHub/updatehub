// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

#[cfg(test)]
mod test;

pub(crate) mod download_abort;
pub(crate) mod info;
pub(crate) mod local_install;
pub(crate) mod probe;
/// Used to send `Step` messages to the `Machine` actor.
pub(crate) mod stepper;

use super::{
    Idle, Metadata, PrepareLocalInstall, Probe, RuntimeSettings, Settings, State, StateMachine,
};
use actix::{Actor, Addr, Arbiter, AsyncContext, Context, Handler, Message, ResponseFuture};
use slog_scope::info;

pub(crate) struct Machine {
    state: Option<StateMachine>,
    shared_state: SharedState,
    stepper: stepper::Controller,
}

#[derive(Debug, PartialEq)]
pub(super) struct SharedState {
    pub(super) settings: Settings,
    pub(super) runtime_settings: RuntimeSettings,
    pub(super) firmware: Metadata,
}

impl SharedState {
    pub(super) fn server_address(&self) -> &str {
        self.runtime_settings
            .custom_server_address()
            .unwrap_or(&self.settings.network.server_address)
    }
}

impl Actor for Machine {
    type Context = Context<Self>;

    fn started(&mut self, _: &mut Self::Context) {
        info!("Starting State Machine Actor...");
    }

    fn stopped(&mut self, _: &mut Self::Context) {
        info!("Stopping State Machine Actor...");
    }
}

impl Machine {
    pub(super) fn new(
        state: StateMachine,
        settings: Settings,
        runtime_settings: RuntimeSettings,
        firmware: Metadata,
    ) -> Self {
        Machine {
            state: Some(state),
            shared_state: SharedState { settings, runtime_settings, firmware },
            stepper: stepper::Controller::default(),
        }
    }

    pub(super) fn start(mut self) -> Addr<Self> {
        Machine::start_in_arbiter(&Arbiter::new(), move |ctx| {
            self.stepper.start(ctx.address());
            self
        })
    }
}

#[derive(Message)]
#[rtype(StepTransition)]
struct Step;

pub(crate) enum StepTransition {
    Delayed(std::time::Duration),
    Immediate,
    Never,
}

impl Handler<Step> for Machine {
    type Result = ResponseFuture<StepTransition>;

    fn handle(&mut self, _: Step, _: &mut Context<Self>) -> Self::Result {
        if let Some(machine) = self.state.take() {
            let this: *mut Self = self;

            return Box::pin(async move {
                unsafe {
                    let (state, transition) = machine
                        .move_to_next_state(&mut (*this).shared_state)
                        .await
                        .unwrap_or_else(|e| (StateMachine::from(e), StepTransition::Immediate));
                    (*this).state = Some(state);

                    transition
                }
            });
        }

        unreachable!("Failed to take StateMachine from StateAgent")
    }
}
