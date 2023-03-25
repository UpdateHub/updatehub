// Copyright (C) 2018, 2019, 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

//! Used to communicate with UpdateHub and listen to appropriate callbacks
//! when the state changes.

use crate::{Error, Result};
use log::warn;
use std::{
    collections::HashMap, env, fs, future::Future, io, path::Path, pin::Pin, result, str::FromStr,
    sync::Arc,
};
use tokio::{
    io::{AsyncBufReadExt, AsyncWriteExt, BufReader},
    net::{UnixListener, UnixStream},
    sync::Mutex,
};

const SDK_TRIGGER_FILENAME: &str =
    "/usr/share/updatehub/state-change-callbacks.d/10-updatehub-sdk-statechange-trigger";
const SOCKET_PATH: &str = "/run/updatehub-statechange.sock";

type CallbackFn = dyn Fn(Handler) -> Pin<Box<dyn Future<Output = Result<()>>>>;

/// Struct that store the callbacks for the states.
#[derive(Default)]
pub struct StateChange {
    callbacks: HashMap<State, Vec<Box<CallbackFn>>>,
}

/// The state of the agent that can be handled.
#[derive(Debug, PartialEq, Eq, Hash)]
pub enum State {
    Probe,
    Download,
    Install,
    Reboot,
    Error,
}

impl FromStr for State {
    type Err = io::Error;

    fn from_str(s: &str) -> result::Result<Self, Self::Err> {
        match s {
            "probe" => Ok(State::Probe),
            "download" => Ok(State::Download),
            "install" => Ok(State::Install),
            "reboot" => Ok(State::Reboot),
            "error" => Ok(State::Error),
            _ => Err(io::Error::new(
                io::ErrorKind::InvalidInput,
                format!("the '{}' is not a valid state", s),
            )),
        }
    }
}

/// Handler used to communicate with UpdateHub
/// to call commands on the state callbacks.
pub struct Handler {
    stream: Arc<Mutex<UnixStream>>,
}

impl Handler {
    /// Cancels the current action on the agent.
    pub async fn cancel(&mut self) -> Result<()> {
        self.stream.lock().await.write_all(b"cancel").await.map_err(Error::Io)
    }

    /// Tell the agent to proceed with the transition.
    pub async fn proceed(&self) -> Result<()> {
        // No message need to be sent to the connection in order to the
        // agent to proceed handling the current state.
        Ok(())
    }
}

impl StateChange {
    /// Creates a new `StateChange` struct.
    #[inline]
    pub fn new() -> Self {
        StateChange::default()
    }

    /// Function that register callback(s) for the state(s).
    /// # Example
    ///
    /// ```no_run
    /// use updatehub_sdk::listener;
    ///
    /// # let mut listener = listener::StateChange::default();
    /// listener.on_state(listener::State::Download, |mut handler| async move {
    ///     println!("function called when starting the Download state");
    ///
    ///     // Cancels the current download.
    ///     handler.cancel().await
    /// });
    /// ```
    pub fn on_state<F, Fut>(&mut self, state: State, f: F)
    where
        F: Fn(Handler) -> Fut + 'static,
        Fut: Future<Output = Result<()>> + 'static,
    {
        self.callbacks.entry(state).or_insert_with(Vec::new).push(Box::new(move |d| Box::pin(f(d))))
    }

    /// Start the agent to listen for messages on the socket.
    pub async fn listen(&self) -> Result<()> {
        let sdk_trigger = Path::new(SDK_TRIGGER_FILENAME);
        if !sdk_trigger.exists() {
            warn!("WARNING: updatehub-sdk-statechange-trigger not found on {:?}", sdk_trigger);
        }

        let socket_path = env::var("UH_LISTENER_TEST").unwrap_or_else(|_| SOCKET_PATH.to_string());
        let socket_path = Path::new(&socket_path);
        if socket_path.exists() {
            fs::remove_file(socket_path)?;
        }

        let listener = UnixListener::bind(socket_path)?;
        loop {
            let (socket, ..) = listener.accept().await?;
            self.handle_connection(socket).await?;
        }
    }

    async fn handle_connection(&self, mut stream: UnixStream) -> Result<()> {
        let mut line = String::new();

        {
            let mut reader = BufReader::new(&mut stream);
            reader.read_line(&mut line).await?;
        }

        self.emit(stream, line.trim()).await
    }

    async fn emit(&self, stream: UnixStream, input: &str) -> Result<()> {
        let state = State::from_str(input)?;
        if let Some(callbacks) = self.callbacks.get(&state) {
            // Since tokio::net::UnixStream doesn implement clone (or try_clone)
            // we use an Arc + Mutex in order to be able to clone it into the handle (hance
            // Arc) and to ensure we can get an mutable reference to it (hance the Mutex)
            let stream = Arc::new(Mutex::new(stream));
            for f in callbacks {
                let stream = stream.clone();
                f(Handler { stream }).await?;
            }
        }

        Ok(())
    }
}
