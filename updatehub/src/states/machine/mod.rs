// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

mod address;

use super::{
    DirectDownload, EntryPoint, Metadata, PrepareLocalInstall, Result, RuntimeSettings, Settings,
    State, StateChangeImpl, Validation,
};
use async_std::{channel, prelude::FutureExt};
use slog_scope::{error, info, trace};
use std::path::PathBuf;

pub(crate) use address::{
    AbortDownloadResponse, Addr, Message, ProbeResponse, Response, StateResponse,
};

pub(super) struct StateMachine {
    state: State,
    context: Context,
}

pub struct Context {
    pub(super) communication: Channel<(Message, channel::Sender<Result<Response>>)>,
    pub(super) waker: Channel<()>,
    pub settings: Settings,
    pub runtime_settings: RuntimeSettings,
    pub firmware: Metadata,
}

pub(super) struct Channel<T> {
    pub(super) sender: channel::Sender<T>,
    pub(super) receiver: channel::Receiver<T>,
}

impl<T> Channel<T> {
    fn new(cap: usize) -> Self {
        let (sender, receiver) = channel::bounded(cap);
        Channel { sender, receiver }
    }
}

impl CommunicationState for State {}

#[async_trait::async_trait]
pub(super) trait CommunicationState: StateChangeImpl {
    async fn handle_communication(
        &self,
        msg: address::Message,
        responder: channel::Sender<Result<address::Response>>,
        context: &mut Context,
    ) -> Option<State> {
        trace!("received external request: {:?}", msg);

        let res = match msg {
            address::Message::Info => {
                let state = self.name().to_owned();
                Ok((
                    address::Response::Info(sdk::api::info::Response {
                        state,
                        version: crate::version().to_string(),
                        config: context.settings.0.clone(),
                        firmware: context.firmware.0.clone(),
                        runtime_settings: context.runtime_settings.inner.clone(),
                    }),
                    None,
                ))
            }
            address::Message::Probe(custom_server) => self
                .handle_probe(context, custom_server)
                .await
                .map(|(res, st)| (address::Response::Probe(res), st)),
            address::Message::AbortDownload => self
                .handle_abort_download(context)
                .await
                .map(|(res, st)| (address::Response::AbortDownload(res), st)),
            address::Message::LocalInstall(update_file) => self
                .handle_local_install(context, update_file)
                .await
                .map(|(res, st)| (address::Response::LocalInstall(res), st)),
            address::Message::RemoteInstall(url) => self
                .handle_remote_install(context, url)
                .await
                .map(|(res, st)| (address::Response::RemoteInstall(res), st)),
        };

        match res {
            Ok((response, state)) => {
                responder.send(Ok(response)).await.ok()?;
                state
            }
            Err(e) => {
                error!("Request failed with: {}", e);
                responder.send(Err(e)).await.ok()?;
                None
            }
        }
    }

    async fn handle_probe(
        &self,
        context: &mut Context,
        custom_server: Option<String>,
    ) -> Result<(address::ProbeResponse, Option<State>)> {
        use chrono::Utc;
        use cloud::api::ProbeResponse;

        if !self.is_preemptive_state() {
            let name = self.name().to_owned();
            return Ok((address::ProbeResponse::Busy(name), None));
        }
        // Starting logging a new scope of operation since we are
        // starting to handle a user request
        crate::logger::start_memory_logging();
        info!("Probing the server as requested by the user");

        if let Some(server_address) = custom_server {
            context.runtime_settings.set_custom_server_address(&server_address);
        }

        match crate::CloudClient::new(context.server_address())
            .probe(context.runtime_settings.retries(), context.firmware.as_cloud_metadata())
            .await?
        {
            ProbeResponse::ExtraPoll(s) => {
                info!("server responded with extra poll of {} seconds", s);
                Ok((address::ProbeResponse::Delayed(s), None))
            }

            ProbeResponse::NoUpdate => {
                info!("no update is current available for this device");
                context.waker.sender.send(()).await?;

                // Store timestamp of last polling
                context.runtime_settings.set_last_polling(Utc::now())?;
                Ok((address::ProbeResponse::Unavailable, Some(State::EntryPoint(EntryPoint {}))))
            }

            ProbeResponse::Update(package, sign) => {
                info!("update received: {} ({})", package.version(), package.package_uid());
                context.waker.sender.send(()).await?;

                // Store timestamp of last polling
                context.runtime_settings.set_last_polling(Utc::now())?;
                Ok((
                    address::ProbeResponse::Available,
                    Some(State::Validation(Validation { package, sign })),
                ))
            }
        }
    }

    async fn handle_abort_download(
        &self,
        _: &Context,
    ) -> Result<(address::AbortDownloadResponse, Option<State>)> {
        if self.is_handling_download() {
            Ok((
                address::AbortDownloadResponse::RequestAccepted,
                Some(State::EntryPoint(EntryPoint {})),
            ))
        } else {
            Ok((address::AbortDownloadResponse::InvalidState, None))
        }
    }

    async fn handle_local_install(
        &self,
        context: &Context,
        update_file: PathBuf,
    ) -> Result<(address::StateResponse, Option<State>)> {
        let name = self.name().to_owned();
        if self.is_preemptive_state() {
            // Starting logging a new scope of operation since we are
            // starting to handle a user request
            crate::logger::start_memory_logging();
            context.waker.sender.send(()).await?;

            Ok((
                address::StateResponse::RequestAccepted(name),
                Some(State::PrepareLocalInstall(PrepareLocalInstall { update_file })),
            ))
        } else {
            Ok((address::StateResponse::InvalidState(name), None))
        }
    }

    async fn handle_remote_install(
        &self,
        context: &Context,
        url: String,
    ) -> Result<(address::StateResponse, Option<State>)> {
        let name = self.name().to_owned();

        if self.is_preemptive_state() {
            // Starting logging a new scope of operation since we are
            // starting to handle a user request
            crate::logger::start_memory_logging();
            context.waker.sender.send(()).await?;

            Ok((
                address::StateResponse::RequestAccepted(name),
                Some(State::DirectDownload(DirectDownload { url })),
            ))
        } else {
            Ok((address::StateResponse::InvalidState(name), None))
        }
    }
}

impl Context {
    pub(crate) fn new(
        settings: Settings,
        runtime_settings: RuntimeSettings,
        firmware: Metadata,
    ) -> Self {
        Context {
            communication: Channel::new(10),
            waker: Channel::new(1),
            settings,
            runtime_settings,
            firmware,
        }
    }

    pub(super) fn server_address(&self) -> &str {
        self.runtime_settings
            .custom_server_address()
            .unwrap_or(&self.settings.network.server_address)
    }
}

#[derive(Debug)]
pub(super) enum StepTransition {
    Delayed(chrono::Duration),
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
        StateMachine { state, context: Context::new(settings, runtime_settings, firmware) }
    }

    pub(super) fn address(&self) -> Addr {
        Addr { message: self.context.communication.sender.clone() }
    }

    pub(super) async fn start(mut self) {
        loop {
            // Since the loop is already currently running, we can
            // discharges any wake message received.
            let _ = self.context.waker.receiver.try_recv();

            self.consume_pending_communication().await;

            let (state, transition) = self
                .state
                .handle(&mut self.context)
                .await
                .unwrap_or_else(|e| (State::from(e), StepTransition::Immediate));
            self.state = state;

            match transition {
                StepTransition::Immediate => {}
                StepTransition::Delayed(t) => {
                    trace!("delaying transition for: {} seconds", t.num_seconds());
                    let waker = self.context.waker.receiver.clone();
                    async_std::task::sleep(t.to_std().unwrap())
                        .race(async {
                            let _ = waker.recv().await;
                        })
                        .race(self.await_communication())
                        .await;
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
            if let Some(new_state) =
                self.state.handle_communication(msg, responder, &mut self.context).await
            {
                self.state = new_state;
            }
        }
    }

    async fn await_communication(&mut self) {
        while let Ok((msg, responder)) = self.context.communication.receiver.recv().await {
            if let Some(new_state) =
                self.state.handle_communication(msg, responder, &mut self.context).await
            {
                self.state = new_state;
            }
        }
    }
}
