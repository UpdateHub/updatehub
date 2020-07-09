// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use async_std::sync;
use std::path::PathBuf;

#[derive(Clone)]
pub(crate) struct Addr {
    pub(super) message: sync::Sender<(Message, sync::Sender<Response>)>,
    pub(super) waker: sync::Sender<()>,
}

#[derive(Debug)]
pub(super) enum Message {
    Info,
    Probe(Option<String>),
    AbortDownload,
    LocalInstall(PathBuf),
    RemoteInstall(String),
}

#[derive(Debug)]
pub(super) enum Response {
    Info(sdk::api::info::Response),
    Probe(super::Result<ProbeResponse>),
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

impl Addr {
    pub(crate) async fn request_info(&self) -> sdk::api::info::Response {
        let (sndr, recv) = sync::channel(1);
        self.message.send((Message::Info, sndr)).await;
        match recv.recv().await {
            Ok(Response::Info(resp)) => resp,
            res => unreachable!("Unexpected response: {:?}", res),
        }
    }

    pub(crate) async fn request_probe(
        &self,
        custom_server: Option<String>,
    ) -> super::Result<ProbeResponse> {
        let (sndr, recv) = sync::channel(1);
        self.message.send((Message::Probe(custom_server), sndr)).await;
        match recv.recv().await {
            Ok(Response::Probe(resp)) => resp,
            res => unreachable!("Unexpected response: {:?}", res),
        }
    }

    pub(crate) async fn request_abort_download(&self) -> AbortDownloadResponse {
        let (sndr, recv) = sync::channel(1);
        self.message.send((Message::AbortDownload, sndr)).await;
        match recv.recv().await {
            Ok(Response::AbortDownload(resp)) => resp,
            res => unreachable!("Unexpected response: {:?}", res),
        }
    }

    pub(crate) async fn request_local_install(&self, path: PathBuf) -> StateResponse {
        let (sndr, recv) = sync::channel(1);
        self.message.send((Message::LocalInstall(path), sndr)).await;
        match recv.recv().await {
            Ok(Response::LocalInstall(resp)) => resp,
            res => unreachable!("Unexpected response: {:?}", res),
        }
    }

    pub(crate) async fn request_remote_install(&self, url: String) -> StateResponse {
        let (sndr, recv) = sync::channel(1);
        self.message.send((Message::RemoteInstall(url), sndr)).await;
        match recv.recv().await {
            Ok(Response::RemoteInstall(resp)) => resp,
            res => unreachable!("Unexpected response: {:?}", res),
        }
    }
}
