// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

mod address;

use super::{
    DirectDownload, EntryPoint, Metadata, PrepareLocalInstall, Result, RuntimeSettings, Settings,
    State, StateChangeImpl, Validation,
};
use async_std::{prelude::FutureExt, sync};
use slog_scope::trace;

pub(crate) use address::{AbortDownloadResponse, Addr, ProbeResponse, StateResponse};

pub(super) struct StateMachine {
    state: State,
    context: Context,
}

pub(super) struct Context {
    communication: Channel<(address::Message, sync::Sender<address::Response>)>,
    waker: Channel<()>,
    shared_state: SharedState,
}

#[derive(Debug, PartialEq)]
pub struct SharedState {
    pub settings: Settings,
    pub runtime_settings: RuntimeSettings,
    pub firmware: Metadata,
}

struct Channel<T> {
    sender: sync::Sender<T>,
    receiver: sync::Receiver<T>,
}

impl<T> Channel<T> {
    fn new(cap: usize) -> Self {
        let (sender, receiver) = sync::channel(cap);
        Channel { sender, receiver }
    }
}

impl SharedState {
    pub(super) fn server_address(&self) -> &str {
        self.runtime_settings
            .custom_server_address()
            .unwrap_or(&self.settings.network.server_address)
    }
}

#[derive(Debug)]
pub(super) enum StepTransition {
    Delayed(std::time::Duration),
    Immediate,
    Never,
}

impl StateMachine {
    pub(super) fn new(
        state: State,
        settings: Settings,
        runtime_settings: RuntimeSettings,
        firmware: Metadata,
    ) -> Self {
        StateMachine {
            state,
            context: Context {
                communication: Channel::new(10),
                waker: Channel::new(1),
                shared_state: SharedState { settings, runtime_settings, firmware },
            },
        }
    }

    pub(super) fn address(&self) -> Addr {
        Addr {
            message: self.context.communication.sender.clone(),
            waker: self.context.waker.sender.clone(),
        }
    }

    pub(super) async fn start(mut self) {
        loop {
            // Since the loop is already currently running, we can
            // discharges any wake message received.
            let _ = self.context.waker.receiver.try_recv();

            self.consume_pending_communication().await;

            let (state, transition) = self
                .state
                .move_to_next_state(&mut self.context.shared_state)
                .await
                .unwrap_or_else(|e| (State::from(e), StepTransition::Immediate));
            self.state = state;

            match transition {
                StepTransition::Immediate => {}
                StepTransition::Delayed(t) => {
                    trace!("delaying transition for: {} seconds", t.as_secs());
                    async_std::task::sleep(t).race(self.await_communication()).await;
                }
                StepTransition::Never => {
                    trace!("stopping transition until awoken");
                    let _ = self
                        .context
                        .waker
                        .receiver
                        .clone()
                        .recv()
                        .race(async {
                            self.await_communication().await;
                            Ok(())
                        })
                        .await;
                }
            }
        }
    }

    async fn consume_pending_communication(&mut self) {
        while let Ok((msg, responder)) = self.context.communication.receiver.try_recv() {
            self.handle_communication(msg, responder).await;
        }
    }

    async fn await_communication(&mut self) {
        while let Ok((msg, responder)) = self.context.communication.receiver.recv().await {
            self.handle_communication(msg, responder).await;
        }
    }

    async fn handle_communication(
        &mut self,
        msg: address::Message,
        responder: sync::Sender<address::Response>,
    ) {
        trace!("Received external request: {:?}", msg);

        let response = match msg {
            address::Message::Info => {
                let state = self.state.name().to_owned();
                address::Response::Info(sdk::api::info::Response {
                    state,
                    version: crate::version().to_string(),
                    config: self.context.shared_state.settings.0.clone(),
                    firmware: self.context.shared_state.firmware.0.clone(),
                    runtime_settings: self.context.shared_state.runtime_settings.0.clone(),
                })
            }
            address::Message::Probe(custom_server) => {
                address::Response::Probe(self.handle_probe_request(custom_server).await)
            }
            address::Message::AbortDownload => {
                if self.state.is_handling_download() {
                    self.state = State::EntryPoint(EntryPoint {});
                    address::Response::AbortDownload(
                        address::AbortDownloadResponse::RequestAccepted,
                    )
                } else {
                    address::Response::AbortDownload(address::AbortDownloadResponse::InvalidState)
                }
            }
            address::Message::LocalInstall(update_file) => {
                let state = self.state.name().to_owned();

                if self.state.is_preemptive_state() {
                    crate::logger::start_memory_logging();
                    self.context.waker.sender.send(()).await;

                    self.state = State::PrepareLocalInstall(PrepareLocalInstall { update_file });

                    address::Response::LocalInstall(address::StateResponse::RequestAccepted(state))
                } else {
                    address::Response::LocalInstall(address::StateResponse::InvalidState(state))
                }
            }
            address::Message::RemoteInstall(url) => {
                let state = self.state.name().to_owned();

                if self.state.is_preemptive_state() {
                    crate::logger::start_memory_logging();
                    self.context.waker.sender.send(()).await;

                    self.state = State::DirectDownload(DirectDownload { url });

                    address::Response::RemoteInstall(address::StateResponse::RequestAccepted(state))
                } else {
                    address::Response::RemoteInstall(address::StateResponse::InvalidState(state))
                }
            }
        };

        responder.send(response).await;
    }

    async fn handle_probe_request(
        &mut self,
        custom_server: Option<String>,
    ) -> Result<address::ProbeResponse> {
        use chrono::Utc;
        use cloud::api::ProbeResponse;

        if !self.state.is_preemptive_state() {
            let state = self.state.name().to_owned();
            return Ok(address::ProbeResponse::Busy(state));
        }

        if let Some(server_address) = custom_server {
            self.context.shared_state.runtime_settings.set_custom_server_address(&server_address);
        }

        match crate::CloudClient::new(&self.context.shared_state.server_address())
            .probe(
                self.context.shared_state.runtime_settings.retries() as u64,
                self.context.shared_state.firmware.as_cloud_metadata(),
            )
            .await?
        {
            ProbeResponse::ExtraPoll(s) => Ok(address::ProbeResponse::Delayed(s)),

            ProbeResponse::NoUpdate => {
                self.context.waker.sender.send(()).await;

                // Store timestamp of last polling
                self.context.shared_state.runtime_settings.set_last_polling(Utc::now())?;
                self.state = State::EntryPoint(EntryPoint {});
                Ok(address::ProbeResponse::Unavailable)
            }

            ProbeResponse::Update(package, sign) => {
                self.context.waker.sender.send(()).await;

                // Store timestamp of last polling
                self.context.shared_state.runtime_settings.set_last_polling(Utc::now())?;
                self.state = State::Validation(Validation { package, sign });
                Ok(address::ProbeResponse::Available)
            }
        }
    }
}
