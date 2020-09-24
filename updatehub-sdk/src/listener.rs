// Copyright (C) 2018, 2019, 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::{Error, Result};
use async_std::{
    io::BufReader,
    os::unix::net::{UnixListener, UnixStream},
    prelude::*,
};
use log::warn;
use std::{env, fs, path::Path, pin::Pin};

const SDK_TRIGGER_FILENAME: &str =
    "/usr/share/updatehub/state-change-callbacks.d/10-updatehub-sdk-statechange-trigger";
const SOCKET_PATH: &str = "/run/updatehub-statechange.sock";

type CallbackFn = dyn Fn(Handler) -> Pin<Box<dyn Future<Output = Result<()>>>>;

#[derive(Default)]
pub struct StateChangeListener {
    download_callbacks: Vec<Box<CallbackFn>>,
    install_callbacks: Vec<Box<CallbackFn>>,
    reboot_callbacks: Vec<Box<CallbackFn>>,
    error_callbacks: Vec<Box<CallbackFn>>,
}

#[derive(Debug)]
pub enum State {
    Download,
    Install,
    Reboot,
    Error,
}

pub struct Handler {
    stream: UnixStream,
}

impl Handler {
    // Cancels the current action on the agent.
    pub async fn cancel(&mut self) -> Result<()> {
        self.stream.write_all(b"cancel").await.map_err(Error::Io)
    }

    // Tell the agent to proceed with the transition.
    pub async fn proceed(&self) -> Result<()> {
        // No message need to be sent to the connection in order to the
        // agent to proceed handling the current state.
        Ok(())
    }
}

impl StateChangeListener {
    #[inline]
    pub fn new() -> Self {
        StateChangeListener {
            download_callbacks: Vec::new(),
            install_callbacks: Vec::new(),
            reboot_callbacks: Vec::new(),
            error_callbacks: Vec::new(),
        }
    }

    pub fn on_state<F, Fut>(&mut self, state: State, f: F)
    where
        F: Fn(Handler) -> Fut + 'static,
        Fut: Future<Output = Result<()>> + 'static,
    {
        match state {
            State::Download => self.download_callbacks.push(Box::new(move |d| Box::pin(f(d)))),
            State::Install => self.install_callbacks.push(Box::new(move |d| Box::pin(f(d)))),
            State::Reboot => self.reboot_callbacks.push(Box::new(move |d| Box::pin(f(d)))),
            State::Error => self.error_callbacks.push(Box::new(move |d| Box::pin(f(d)))),
        }
    }

    pub async fn listen(&self) -> Result<()> {
        let sdk_trigger = Path::new(SDK_TRIGGER_FILENAME);
        if !sdk_trigger.exists() {
            warn!("WARNING: updatehub-sdk-statechange-trigger not found on {:?}", sdk_trigger);
        }

        let socket_path = env::var("UH_LISTENER_TEST").unwrap_or_else(|_| SOCKET_PATH.to_string());
        let socket_path = Path::new(&socket_path);
        if socket_path.exists() {
            fs::remove_file(&socket_path)?;
        }

        let listener = UnixListener::bind(socket_path).await?;
        loop {
            let (socket, ..) = listener.accept().await?;
            self.handle_connection(socket).await?;
        }
    }

    async fn handle_connection(&self, stream: UnixStream) -> Result<()> {
        let mut reader = BufReader::new(&stream);
        let mut line = String::new();

        reader.read_line(&mut line).await?;

        self.emit(stream, &line.trim()).await
    }

    async fn emit(&self, stream: UnixStream, input: &str) -> Result<()> {
        let callbacks = match input {
            "download" => &self.download_callbacks,
            "install" => &self.install_callbacks,
            "reboot" => &self.reboot_callbacks,
            "error" => &self.error_callbacks,
            _ => unreachable!("the input is not valid"),
        };

        for f in callbacks {
            let stream = stream.clone();
            f(Handler { stream }).await?;
        }

        Ok(())
    }
}
