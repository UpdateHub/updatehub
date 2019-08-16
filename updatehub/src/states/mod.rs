// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

#[macro_use]
mod macros;
pub(crate) mod actor;
mod download;
mod error;
mod idle;
pub(crate) mod install;
mod park;
mod poll;
mod prepare_download;
mod probe;
mod reboot;
mod transition;

use self::{
    download::Download,
    error::Error,
    idle::Idle,
    install::Install,
    park::Park,
    poll::Poll,
    prepare_download::PrepareDownload,
    probe::{Probe, ServerAddress},
    reboot::Reboot,
};
use crate::{firmware::Metadata, http_api, runtime_settings::RuntimeSettings, settings::Settings};
use actix::System;
use slog_scope::info;

trait StateChangeImpl {
    fn handle(self, shared_state: &mut SharedState) -> Result<StateMachine, failure::Error>;
    fn name(&self) -> &'static str;

    fn handle_download_abort(&self) -> actor::download_abort::Response {
        actor::download_abort::Response::InvalidState
    }

    fn handle_trigger_probe(&self) -> actor::probe::Response {
        actor::probe::Response::InvalidState(self.name().to_owned())
    }
}

trait TransitionCallback: StateChangeImpl + Into<State<Idle>> {}

trait ProgressReporter: TransitionCallback {
    fn package_uid(&self) -> String;
    fn report_enter_state_name(&self) -> &'static str;
    fn report_leave_state_name(&self) -> &'static str;
}

#[derive(Debug, PartialEq)]
struct State<S>(S)
where
    State<S>: StateChangeImpl;

#[derive(Debug, PartialEq)]
struct SharedState {
    settings: Settings,
    runtime_settings: RuntimeSettings,
    firmware: Metadata,
}

#[derive(Debug, PartialEq)]
enum StateMachine {
    Park(State<Park>),
    Idle(State<Idle>),
    Poll(State<Poll>),
    Probe(State<Probe>),
    PrepareDownload(State<PrepareDownload>),
    Download(State<Download>),
    Install(State<Install>),
    Reboot(State<Reboot>),
    Error(State<Error>),
}

impl<S> State<S>
where
    State<S>: TransitionCallback + ProgressReporter,
{
    fn handle_with_callback_and_report_progress(
        self,
        shared_state: &mut SharedState,
    ) -> Result<StateMachine, failure::Error> {
        use transition::{state_change_callback, Transition};

        let transition =
            state_change_callback(&shared_state.settings.firmware.metadata_path, self.name())?;

        match transition {
            Transition::Continue => Ok(self.handle_and_report_progress(shared_state)?),
            Transition::Cancel => Ok(StateMachine::Idle(self.into())),
        }
    }
}

impl<S> State<S>
where
    State<S>: ProgressReporter,
{
    fn handle_and_report_progress(
        self,
        shared_state: &mut SharedState,
    ) -> Result<StateMachine, failure::Error> {
        let server = &shared_state.settings.network.server_address.clone();
        let firmware = &shared_state.firmware.clone();
        let package_uid = &self.package_uid();
        let enter_state = self.report_enter_state_name();
        let leave_state = self.report_leave_state_name();

        let report = |state, previous_state, error_message, current_log| {
            crate::client::Api::new(&server).report(
                state,
                firmware,
                package_uid,
                previous_state,
                error_message,
                current_log,
            )
        };

        report(enter_state, None, None, None)?;
        self.handle(shared_state)
            .and_then(|state| {
                report(leave_state, None, None, None)?;
                Ok(state)
            })
            .or_else(|e| {
                report(
                    "error",
                    Some(enter_state),
                    Some(e.to_string()),
                    Some(crate::logger::buffer().lock().unwrap().to_string()),
                )?;
                Err(e)
            })
    }
}

impl StateMachine {
    fn new() -> Self {
        StateMachine::Idle(State(Idle {}))
    }

    fn move_to_next_state(self, shared_state: &mut SharedState) -> Result<Self, failure::Error> {
        match self {
            StateMachine::Error(s) => Ok(s.handle(shared_state)?),
            StateMachine::Park(s) => Ok(s.handle(shared_state)?),
            StateMachine::Idle(s) => Ok(s.handle(shared_state)?),
            StateMachine::Poll(s) => Ok(s.handle(shared_state)?),
            StateMachine::Probe(s) => Ok(s.handle(shared_state)?),
            StateMachine::PrepareDownload(s) => Ok(s.handle(shared_state)?),
            StateMachine::Download(s) => {
                Ok(s.handle_with_callback_and_report_progress(shared_state)?)
            }
            StateMachine::Install(s) => {
                Ok(s.handle_with_callback_and_report_progress(shared_state)?)
            }
            StateMachine::Reboot(s) => Ok(s.handle_with_callback_and_report_progress(shared_state)?),
        }
    }

    fn for_any_state<F, A>(&self, f: F) -> A
    where
        F: Fn(&dyn StateChangeImpl) -> A,
    {
        match self {
            StateMachine::Error(s) => f(s),
            StateMachine::Park(s) => f(s),
            StateMachine::Idle(s) => f(s),
            StateMachine::Poll(s) => f(s),
            StateMachine::Probe(s) => f(s),
            StateMachine::PrepareDownload(s) => f(s),
            StateMachine::Download(s) => f(s),
            StateMachine::Install(s) => f(s),
            StateMachine::Reboot(s) => f(s),
        }
    }
}

/// Runs the state machine up to completion handling all procing
/// states without extra manual work.
///
/// It supports following states, and transitions, as shown in the
/// below diagram:
///
/// ```text
///           .--------------.
///           |              v
/// Park <- Idle -> Poll -> Probe -> Download -> Install -> Reboot
///           ^      ^        '          '          '
///           '      '        '          '          '
///           '      `--------'          '          '
///           `---------------'          '          '
///           `--------------------------'          '
///           `-------------------------------------'
/// ```
///
/// # Example
/// ```
/// # extern crate failure;
/// # extern crate updatehub;
/// # use failure;
/// # fn run() -> Result<(), failure::Error> {
/// use updatehub;
///
/// updatehub::logger::init(0);
/// let settings = updatehub::Settings::load()?;
/// updatehub::run(settings)?;
/// # Ok(())
/// # }
/// ```
pub fn run(settings: Settings) -> Result<(), failure::Error> {
    let listen_socket = settings.network.listen_socket.clone();
    let mut runtime_settings = RuntimeSettings::new().load(&settings.storage.runtime_settings)?;
    if !settings.storage.read_only {
        runtime_settings.enable_persistency();
    }
    let firmware = Metadata::from_path(&settings.firmware.metadata_path)?;

    System::run(move || {
        let machine_addr = actor::Machine::start(
            StateMachine::new(),
            SharedState {
                settings,
                runtime_settings,
                firmware,
            },
        );

        actix_web::HttpServer::new(move || {
            actix_web::App::new()
                .configure(|cfg| http_api::API::configure(cfg, machine_addr.clone()))
        })
        .bind(listen_socket.clone())
        .unwrap_or_else(|_| {
            panic!(
                "Failed to bind listen socket, {:?}, for HTTP API",
                listen_socket,
            )
        })
        .start();
    })?;

    info!("actix System has stopped");
    Ok(())
}
