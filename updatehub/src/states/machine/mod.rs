// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

mod address;

#[cfg(test)]
mod tests;

use super::{
    DirectDownload, EntryPoint, Metadata, PrepareLocalInstall, Result, RuntimeSettings, Settings,
    State, StateChangeImpl, Validation,
};
use async_lock::Mutex;
use futures_util::future::{Either, select};
use slog_scope::{error, info, trace};
use std::{future::Future, path::PathBuf};

pub(crate) use address::{
    AbortDownloadResponse, Addr, Message, ProbeResponse, Response, StateResponse,
};

pub(super) struct StateMachine {
    state: State,
    context: Context,
}

pub struct Context {
    pub(super) communication: Channel<(Message, async_channel::Sender<Result<Response>>)>,
    pub(super) waker: Channel<()>,
    pub settings: Settings,
    pub runtime_settings: RuntimeSettings,
    pub firmware: Metadata,
}

pub(super) struct Channel<T> {
    pub(super) sender: async_channel::Sender<T>,
    pub(super) receiver: async_channel::Receiver<T>,
}

impl<T> Channel<T> {
    fn new(cap: usize) -> Self {
        let (sender, receiver) = async_channel::bounded(cap);
        Channel { sender, receiver }
    }
}

impl CommunicationState for State {}

pub(super) trait CommunicationState: StateChangeImpl {
    async fn handle_communication(
        &self,
        msg: address::Message,
        responder: async_channel::Sender<Result<address::Response>>,
        context: &mut Context,
    ) -> Option<State> {
        trace!("received external request: {:?}", msg);

        let res = match msg {
            address::Message::Info => {
                let state = self.name().to_owned();
                Ok((
                    address::Response::Info(Box::new(sdk::api::info::Response {
                        state,
                        version: crate::version().to_string(),
                        config: context.settings.0.clone(),
                        firmware: context.firmware.0.clone(),
                        runtime_settings: context.runtime_settings.inner.clone(),
                    })),
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

        let (reply, state) = match res {
            Ok((response, state)) => (Ok(response), state),
            Err(e) => {
                error!("Request failed with: {}", e);
                (Err(e), None)
            }
        };

        // A client that hung up before we answered is tolerated, but the work
        // already done on its behalf is not thrown away: an accepted request
        // still moves the machine to the state it asked for.
        if responder.send(reply).await.is_err() {
            trace!("client gave up before the response was delivered");
        }

        state
    }

    /// Runs `work` while answering requests, returning whichever settles first:
    /// the state `work` produced, or the state a request moved the machine to.
    ///
    /// A request already being handled is never dropped, whichever way the
    /// races below go: dropping one leaves its caller waiting on a reply
    /// channel nobody holds any more, and the caller answers that with an
    /// error the request never recovers from.
    ///
    /// `work` must not hold the context lock across an await. Answering a
    /// request takes that lock, and `work` is not polled while the lock is
    /// being acquired, so a guard held over a suspension point deadlocks both.
    async fn handle_communication_while(
        &self,
        context: &Mutex<&mut Context>,
        work: impl Future<Output = Result<State>>,
    ) -> Result<State> {
        let communication = context.lock().await.communication.receiver.clone();
        futures_util::pin_mut!(work);

        loop {
            let received = communication.recv();
            futures_util::pin_mut!(received);

            let (msg, responder) = match select(work.as_mut(), received).await {
                Either::Left((done, _)) => return done,
                Either::Right((Ok(request), _)) => request,
                // Nobody can ask us anything any more, so just finish the work.
                Either::Right((Err(_), _)) => return work.as_mut().await,
            };

            let mut locked = context.lock().await;
            let handling = self.handle_communication(msg, responder, *locked);
            futures_util::pin_mut!(handling);

            // `work` keeps being polled while the request is answered, but the
            // answering itself is never what gets dropped: when `work` wins
            // this race it hands back the unfinished `handling`, which is then
            // driven to completion rather than discarded.
            match select(work.as_mut(), handling).await {
                // The request still gets its answer, and a request that
                // redirects the machine outranks work we no longer want.
                Either::Left((done, handling)) => return handling.await.map_or(done, Ok),
                Either::Right((Some(new_state), _)) => return Ok(new_state),
                Either::Right((None, _)) => {}
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
                context.wake();

                // Store timestamp of last polling
                context.runtime_settings.set_last_polling(Utc::now())?;
                Ok((address::ProbeResponse::Unavailable, Some(State::EntryPoint(EntryPoint {}))))
            }

            ProbeResponse::Update(package, sign) => {
                info!("update received: {} ({})", package.version(), package.package_uid());
                context.wake();

                // Store timestamp of last polling
                context.runtime_settings.set_last_polling(Utc::now())?;
                Ok((
                    address::ProbeResponse::Available,
                    Some(State::Validation(Validation { package, sign, require_download: true })),
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
            context.wake();

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
            context.wake();

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

    /// Asks the main loop to stop waiting on the current transition.
    ///
    /// The waker holds a single pending wake up, so an occupied slot already
    /// carries the message. Signalling must never block: a handler runs to
    /// completion before the loop that drains the waker gets to run again, so
    /// waiting for room here would deadlock the agent.
    pub(super) fn wake(&self) {
        let _ = self.waker.sender.try_send(());
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
        let waker = self.context.waker.receiver.clone();
        let communication = self.context.communication.receiver.clone();

        loop {
            // Since the loop is already currently running, we can
            // discharges any wake message received.
            let _ = waker.try_recv();

            self.consume_pending_communication().await;

            let (state, transition) = self
                .state
                .handle(&mut self.context)
                .await
                .unwrap_or_else(|e| (State::from(e), StepTransition::Immediate));
            self.state = state;

            let delay = match transition {
                StepTransition::Immediate => continue,
                StepTransition::Delayed(t) => {
                    trace!("delaying transition for: {} seconds", t.num_seconds());
                    Some(t.to_std().unwrap_or_default())
                }
                StepTransition::Never => {
                    trace!("stopping transition until awoken");
                    None
                }
            };

            // Built once and polled across every request answered below, so
            // answering one does not restart the wait the current state asked
            // for. `Never` is the same wait with a deadline that never fires.
            let mut elapsed = std::pin::pin!(async {
                match delay {
                    Some(t) => tokio::time::sleep(t).await,
                    None => std::future::pending::<()>().await,
                }
            });

            loop {
                let (msg, responder) = tokio::select! {
                    // Polled in the order the previous `select` chain used.
                    biased;
                    () = elapsed.as_mut() => break,
                    _ = waker.recv() => break,
                    received = communication.recv() => match received {
                        Ok(request) => request,
                        Err(_) => break,
                    },
                };

                // Handled out here on purpose. Only the receive takes part in
                // the race above: dropping a pending receive loses nothing,
                // whereas dropping a request already being handled would leave
                // its caller waiting on a reply channel nobody holds any more.
                if self.handle_request(msg, responder).await {
                    // The request moved the machine, so the wait it was made
                    // against no longer describes what to do next.
                    break;
                }
            }
        }
    }

    async fn consume_pending_communication(&mut self) {
        while let Ok((msg, responder)) = self.context.communication.receiver.try_recv() {
            let _ = self.handle_request(msg, responder).await;
        }
    }

    /// Answers one request, reporting whether it moved the machine.
    async fn handle_request(
        &mut self,
        msg: Message,
        responder: async_channel::Sender<Result<Response>>,
    ) -> bool {
        match self.state.handle_communication(msg, responder, &mut self.context).await {
            Some(new_state) => {
                self.state = new_state;
                true
            }
            None => false,
        }
    }
}
