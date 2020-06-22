// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{EntryPoint, State, StateMachine, Validation};
use actix::{fut::WrapFuture, Addr, AsyncContext, AtomicResponse, Context, Handler, Message};
use chrono::Utc;
use cloud::api::ProbeResponse;

#[derive(Message)]
#[rtype(result = "super::Result<Response>")]
pub(crate) struct Request(pub(crate) Option<String>);

pub(crate) enum Response {
    Available,
    Unavailable,
    Delayed(i64),
    Busy(String),
}

impl Handler<Request> for super::Machine {
    type Result = AtomicResponse<Self, super::Result<Response>>;

    fn handle(&mut self, req: Request, ctx: &mut Context<Self>) -> Self::Result {
        let addr = ctx.address();
        let this: *mut Self = self;

        AtomicResponse::new(Box::pin(
            async move {
                let this = unsafe { this.as_mut().unwrap() };
                this.external_probe(addr, req.0).await
            }
            .into_actor(self),
        ))
    }
}

impl super::Machine {
    async fn external_probe(
        &mut self,
        addr: Addr<Self>,
        custom_server: Option<String>,
    ) -> super::Result<Response> {
        let machine = self.state.as_ref().expect("Failed to take StateMachine's ownership");

        if machine.for_current_state(|s| s.is_preemptive_state()) {
            self.shared_state.runtime_settings.reset_transient_settings();
            if let Some(server_address) = custom_server {
                self.shared_state.runtime_settings.set_custom_server_address(&server_address);
            }

            return match crate::CloudClient::new(&self.shared_state.server_address())
                .probe(
                    self.shared_state.runtime_settings.retries() as u64,
                    self.shared_state.firmware.as_cloud_metadata(),
                )
                .await?
            {
                ProbeResponse::ExtraPoll(s) => Ok(Response::Delayed(s)),

                ProbeResponse::NoUpdate => {
                    // Store timestamp of last polling
                    self.shared_state.runtime_settings.set_last_polling(Utc::now())?;
                    self.stepper.restart(addr);
                    self.state.replace(StateMachine::EntryPoint(State(EntryPoint {})));
                    Ok(Response::Unavailable)
                }

                ProbeResponse::Update(package, sign) => {
                    // Store timestamp of last polling
                    self.shared_state.runtime_settings.set_last_polling(Utc::now())?;
                    self.stepper.restart(addr);
                    self.state
                        .replace(StateMachine::Validation(State(Validation { package, sign })));
                    Ok(Response::Available)
                }
            };
        }

        let state = machine.for_current_state(|s| s.name().to_owned());
        Ok(Response::Busy(state))
    }
}
