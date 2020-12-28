// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use async_std::channel;
use std::path::PathBuf;

#[derive(Clone)]
pub(crate) struct Addr {
    pub(super) message: channel::Sender<(Message, channel::Sender<super::Result<Response>>)>,
    pub(super) waker: channel::Sender<()>,
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
    Info(sdk::api::info::Response),
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

impl<T> From<channel::SendError<T>> for crate::states::TransitionError {
    fn from(err: channel::SendError<T>) -> Self {
        unreachable!("Unexpected sending error for {:?}", err)
    }
}

impl Addr {
    pub(crate) async fn request_info(&self) -> super::Result<sdk::api::info::Response> {
        let (sndr, recv) = channel::bounded(1);
        self.message.send((Message::Info, sndr)).await?;
        match recv.recv().await {
            Ok(Ok(Response::Info(resp))) => Ok(resp),
            Ok(Err(e)) => Err(e),
            res => unreachable!("Unexpected response: {:?}", res),
        }
    }

    pub(crate) async fn request_probe(
        &self,
        custom_server: Option<String>,
    ) -> super::Result<ProbeResponse> {
        let (sndr, recv) = channel::bounded(1);
        self.message.send((Message::Probe(custom_server), sndr)).await?;
        match recv.recv().await {
            Ok(Ok(Response::Probe(resp))) => Ok(resp),
            Ok(Err(e)) => Err(e),
            res => unreachable!("Unexpected response: {:?}", res),
        }
    }

    pub(crate) async fn request_abort_download(&self) -> super::Result<AbortDownloadResponse> {
        let (sndr, recv) = channel::bounded(1);
        self.message.send((Message::AbortDownload, sndr)).await?;
        match recv.recv().await {
            Ok(Ok(Response::AbortDownload(resp))) => Ok(resp),
            Ok(Err(e)) => Err(e),
            res => unreachable!("Unexpected response: {:?}", res),
        }
    }

    pub(crate) async fn request_local_install(
        &self,
        path: PathBuf,
    ) -> super::Result<StateResponse> {
        let (sndr, recv) = channel::bounded(1);
        self.message.send((Message::LocalInstall(path), sndr)).await?;
        match recv.recv().await {
            Ok(Ok(Response::LocalInstall(resp))) => Ok(resp),
            Ok(Err(e)) => Err(e),
            res => unreachable!("Unexpected response: {:?}", res),
        }
    }

    pub(crate) async fn request_remote_install(&self, url: String) -> super::Result<StateResponse> {
        let (sndr, recv) = channel::bounded(1);
        self.message.send((Message::RemoteInstall(url), sndr)).await?;
        match recv.recv().await {
            Ok(Ok(Response::RemoteInstall(resp))) => Ok(resp),
            Ok(Err(e)) => Err(e),
            res => unreachable!("Unexpected response: {:?}", res),
        }
    }
}
