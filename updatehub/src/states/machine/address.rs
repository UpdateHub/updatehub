// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use slog_scope::trace;
use std::path::PathBuf;

#[derive(Clone)]
pub(crate) struct Addr {
    pub(super) message:
        async_channel::Sender<(Message, async_channel::Sender<super::Result<Response>>)>,
}

#[derive(Debug)]
pub(crate) enum Message {
    Info,
    Probe(Option<String>),
    AbortDownload,
    LocalInstall(PathBuf),
    RemoteInstall(String),
}

#[derive(Debug)]
pub(crate) enum Response {
    Info(Box<sdk::api::info::Response>),
    Probe(ProbeResponse),
    AbortDownload(AbortDownloadResponse),
    LocalInstall(StateResponse),
    RemoteInstall(StateResponse),
}

#[derive(Debug)]
pub(crate) enum ProbeResponse {
    Available,
    Unavailable,
    Delayed(i64),
    Busy(String),
}

#[derive(Debug)]
pub(crate) enum AbortDownloadResponse {
    RequestAccepted,
    InvalidState,
}

#[derive(Debug)]
pub(crate) enum StateResponse {
    RequestAccepted(String),
    InvalidState(String),
}

// Either half of a request's channel going away means the state machine is no
// longer running. Report that rather than taking the caller's task down with a
// panic.
impl<T> From<async_channel::SendError<T>> for crate::states::TransitionError {
    fn from(_: async_channel::SendError<T>) -> Self {
        crate::states::TransitionError::CommunicationFailed
    }
}

impl From<async_channel::RecvError> for crate::states::TransitionError {
    fn from(_: async_channel::RecvError) -> Self {
        crate::states::TransitionError::CommunicationFailed
    }
}

impl Addr {
    /// Hands `msg` to the state machine and waits for the answer to come back.
    async fn request(&self, msg: Message) -> super::Result<Response> {
        let (sndr, recv) = async_channel::bounded(1);
        self.message.send((msg, sndr)).await?;
        recv.recv().await?
    }

    pub(crate) async fn request_info(&self) -> super::Result<sdk::api::info::Response> {
        match self.request(Message::Info).await? {
            Response::Info(resp) => Ok(*resp),
            res => unreachable!("Unexpected response: {res:?}"),
        }
    }

    pub(crate) async fn request_probe(
        &self,
        custom_server: Option<String>,
    ) -> super::Result<ProbeResponse> {
        match self.request(Message::Probe(custom_server)).await? {
            Response::Probe(resp) => Ok(resp),
            res => unreachable!("Unexpected response: {res:?}"),
        }
    }

    pub(crate) async fn request_abort_download(&self) -> super::Result<AbortDownloadResponse> {
        match self.request(Message::AbortDownload).await? {
            Response::AbortDownload(resp) => Ok(resp),
            res => unreachable!("Unexpected response: {res:?}"),
        }
    }

    pub(crate) async fn request_local_install(
        &self,
        path: PathBuf,
    ) -> super::Result<StateResponse> {
        trace!("Local install requested");
        match self.request(Message::LocalInstall(path)).await? {
            Response::LocalInstall(resp) => Ok(resp),
            res => unreachable!("Unexpected response: {res:?}"),
        }
    }

    pub(crate) async fn request_remote_install(&self, url: String) -> super::Result<StateResponse> {
        trace!("Remote install requested");
        match self.request(Message::RemoteInstall(url)).await? {
            Response::RemoteInstall(resp) => Ok(resp),
            res => unreachable!("Unexpected response: {res:?}"),
        }
    }
}
