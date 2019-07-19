// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

#[macro_use]
mod macros;
pub(crate) mod actor;
mod download;
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
    idle::Idle,
    install::Install,
    park::Park,
    poll::Poll,
    prepare_download::PrepareDownload,
    probe::{Probe, ServerAddress},
    reboot::Reboot,
};
use crate::{firmware::Metadata, http_api, runtime_settings::RuntimeSettings, settings::Settings};
use actix::{Actor, System};
use futures::future::Future;
use lazy_static::lazy_static;
use std::sync::{Arc, RwLock};

lazy_static! {
    static ref SHARED_STATE: Arc<RwLock<Option<SharedState>>> = Arc::new(RwLock::new(None));
}

trait StateChangeImpl {
    fn handle(self) -> Result<StateMachine, failure::Error>;
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
}

impl<S> State<S>
where
    State<S>: TransitionCallback + ProgressReporter,
{
    fn handle_with_callback_and_report_progress(self) -> Result<StateMachine, failure::Error> {
        use transition::{state_change_callback, Transition};

        let transition = state_change_callback(
            &(shared_state_mut!().settings.firmware.metadata_path.clone()),
            self.name(),
        )?;

        match transition {
            Transition::Continue => Ok(self.handle_and_report_progress()?),
            Transition::Cancel => Ok(StateMachine::Idle(self.into())),
        }
    }
}

impl<S> State<S>
where
    State<S>: ProgressReporter,
{
    fn handle_and_report_progress(self) -> Result<StateMachine, failure::Error> {
        let server = &shared_state_mut!().settings.network.server_address.clone();
        let firmware = &shared_state_mut!().firmware.clone();
        let package_uid = self.package_uid().clone();
        let enter_state = self.report_enter_state_name();
        let leave_state = self.report_leave_state_name();

        let report = |state, previous_state, error_message, current_log| {
            crate::client::Api::new(&server).report(
                state,
                &firmware,
                &package_uid,
                previous_state,
                error_message,
                current_log,
            )
        };

        report(enter_state, None, None, None)?;
        self.handle()
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

    fn move_to_next_state(self) -> Result<Self, failure::Error> {
        match self {
            StateMachine::Park(s) => Ok(s.handle()?),
            StateMachine::Idle(s) => Ok(s.handle()?),
            StateMachine::Poll(s) => Ok(s.handle()?),
            StateMachine::Probe(s) => Ok(s.handle()?),
            StateMachine::PrepareDownload(s) => Ok(s.handle()?),
            StateMachine::Download(s) => Ok(s.handle_with_callback_and_report_progress()?),
            StateMachine::Install(s) => Ok(s.handle_with_callback_and_report_progress()?),
            StateMachine::Reboot(s) => Ok(s.handle_with_callback_and_report_progress()?),
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
    let mut runtime_settings = RuntimeSettings::new().load(&settings.storage.runtime_settings)?;
    if !settings.storage.read_only {
        runtime_settings.enable_persistency();
    }

    let firmware = Metadata::from_path(&settings.firmware.metadata_path)?;
    set_shared_state!(settings, runtime_settings, firmware);

    let agent_machine = actor::Machine::new(StateMachine::new());

    System::run(|| {
        let machine = agent_machine.start();
        let api = machine.clone();
        actix_web::HttpServer::new(move || {
            actix_web::App::new().configure(|cfg| http_api::API::configure(cfg, api.clone()))
        })
        .bind("localhost:8080")
        .unwrap()
        .start();

        // Iterate over the state machine.
        loop {
            machine
                .send(actor::Step)
                .wait()
                .expect("Failed to communicate with actor");
        }
    })?;

    unreachable!("actix System has stopped");
}
