// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

#[macro_use]
mod macros;
pub(crate) mod actor;
mod direct_download;
mod download;
mod error;
mod idle;
pub(crate) mod install;
mod park;
mod poll;
mod prepare_download;
mod prepare_local_install;
mod probe;
mod reboot;
mod transition;

use self::{
    direct_download::DirectDownload, download::Download, error::Error, idle::Idle,
    install::Install, park::Park, poll::Poll, prepare_download::PrepareDownload,
    prepare_local_install::PrepareLocalInstall, probe::Probe, reboot::Reboot,
};
use crate::{firmware::Metadata, http_api, runtime_settings::RuntimeSettings, settings::Settings};
use async_trait::async_trait;
use derive_more::{Display, From};
use slog_scope::{info, warn};
use std::sync::mpsc;

pub type Result<T> = std::result::Result<T, TransitionError>;

#[derive(Debug, Display, From)]
pub enum TransitionError {
    #[display(fmt = "Request to external link failed")]
    InvalidRequest,
    #[display(fmt = "Not all objects are ready for use")]
    ObjectsNotReady,

    #[display(fmt = "Failed to read from channel: {}", _0)]
    MpscRecv(mpsc::TryRecvError),
    #[display(fmt = "Client error: {}", _0)]
    Client(crate::client::Error),
    #[display(fmt = "Firmware error: {}", _0)]
    Firmware(crate::firmware::Error),
    #[display(fmt = "Installation error: {}", _0)]
    Installation(crate::object::Error),
    #[display(fmt = "Runtime settings error: {}", _0)]
    RuntimeSettings(crate::runtime_settings::Error),
    #[display(fmt = "Update package error: {}", _0)]
    UpdatePackage(crate::update_package::Error),
    #[display(fmt = "Uncompress error: {}", _0)]
    Uncompress(compress_tools::Error),
    #[display(fmt = "Serde error: {}", _0)]
    SerdeJson(serde_json::error::Error),

    #[display(fmt = "Update package error: {}", _0)]
    Io(std::io::Error),
    #[display(fmt = "Mailbox error: {}", _0)]
    ActixMailbox(actix::MailboxError),
    #[display(fmt = "Process error: {}", _0)]
    Process(easy_process::Error),
}

#[async_trait]
trait StateChangeImpl {
    async fn handle(
        self,
        shared_state: &mut actor::SharedState,
    ) -> Result<(StateMachine, actor::StepTransition)>;

    fn name(&self) -> &'static str;

    fn handle_download_abort(&self) -> actor::download_abort::Response {
        actor::download_abort::Response::InvalidState
    }

    fn handle_trigger_probe(&self) -> actor::probe::Response {
        actor::probe::Response::InvalidState(self.name().to_owned())
    }

    fn handle_local_install(&self) -> actor::local_install::Response {
        actor::local_install::Response::InvalidState(self.name().to_owned())
    }

    fn handle_remote_install(&self) -> actor::remote_install::Response {
        actor::remote_install::Response::InvalidState(self.name().to_owned())
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
enum StateMachine {
    Park(State<Park>),
    Idle(State<Idle>),
    Poll(State<Poll>),
    Probe(State<Probe>),
    PrepareDownload(State<PrepareDownload>),
    DirectDownload(State<DirectDownload>),
    PrepareLocalInstall(State<PrepareLocalInstall>),
    Download(State<Download>),
    Install(State<Install>),
    Reboot(State<Reboot>),
    Error(State<Error>),
}

impl<S> State<S>
where
    State<S>: TransitionCallback + ProgressReporter,
{
    async fn handle_with_callback_and_report_progress(
        self,
        shared_state: &mut actor::SharedState,
    ) -> Result<(StateMachine, actor::StepTransition)> {
        use transition::{state_change_callback, Transition};

        let transition =
            state_change_callback(&shared_state.settings.firmware.metadata, self.name())?;

        match transition {
            Transition::Continue => Ok(self.handle_and_report_progress(shared_state).await?),
            Transition::Cancel => {
                Ok((StateMachine::Idle(self.into()), actor::StepTransition::Immediate))
            }
        }
    }
}

impl<S> State<S>
where
    State<S>: ProgressReporter,
{
    async fn handle_and_report_progress(
        self,
        shared_state: &mut actor::SharedState,
    ) -> Result<(StateMachine, actor::StepTransition)> {
        let server = shared_state.server_address().to_owned();
        let firmware = &shared_state.firmware.clone();
        let package_uid = &self.package_uid();
        let enter_state = self.report_enter_state_name();
        let leave_state = self.report_leave_state_name();
        let api = crate::client::Api::new(&server);

        let report = |state, previous_state, error_message, current_log| {
            api.report(state, firmware, package_uid, previous_state, error_message, current_log)
        };

        if let Err(e) = report(enter_state, None, None, None).await {
            warn!("Report failed: {}", e);
        }
        match self.handle(shared_state).await {
            Ok((state, trans)) => {
                if let Err(e) = report(leave_state, None, None, None).await {
                    warn!("Report failed: {}", e);
                };
                Ok((state, trans))
            }
            Err(e) => {
                if let Err(e) = report(
                    "error",
                    Some(enter_state),
                    Some(e.to_string()),
                    Some(crate::logger::buffer().lock().unwrap().to_string()),
                )
                .await
                {
                    warn!("Report failed: {}", e);
                }
                Err(e)
            }
        }
    }
}

impl StateMachine {
    fn new() -> Self {
        StateMachine::Idle(State(Idle {}))
    }

    async fn move_to_next_state(
        self,
        shared_state: &mut actor::SharedState,
    ) -> Result<(Self, actor::StepTransition)> {
        match self {
            StateMachine::Error(s) => s.handle(shared_state).await,
            StateMachine::Park(s) => s.handle(shared_state).await,
            StateMachine::Idle(s) => s.handle(shared_state).await,
            StateMachine::Poll(s) => s.handle(shared_state).await,
            StateMachine::Probe(s) => s.handle(shared_state).await,
            StateMachine::PrepareDownload(s) => s.handle(shared_state).await,
            StateMachine::DirectDownload(s) => s.handle(shared_state).await,
            StateMachine::PrepareLocalInstall(s) => s.handle(shared_state).await,
            StateMachine::Download(s) => {
                s.handle_with_callback_and_report_progress(shared_state).await
            }
            StateMachine::Install(s) => {
                s.handle_with_callback_and_report_progress(shared_state).await
            }
            StateMachine::Reboot(s) => {
                s.handle_with_callback_and_report_progress(shared_state).await
            }
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
            StateMachine::DirectDownload(s) => f(s),
            StateMachine::PrepareLocalInstall(s) => f(s),
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
/// # extern crate updatehub;
/// # async fn run() -> Result<(), updatehub::Error> {
/// use updatehub;
///
/// updatehub::logger::init(slog::Level::Info);
/// let settings = updatehub::Settings::load()?;
/// updatehub::run(settings).await?;
/// # Ok(())
/// # }
/// ```
pub async fn run(settings: Settings) -> crate::Result<()> {
    let listen_socket = settings.network.listen_socket.clone();
    let mut runtime_settings = RuntimeSettings::load(&settings.storage.runtime_settings)?;
    if !settings.storage.read_only {
        runtime_settings.enable_persistency();
    }
    let firmware = Metadata::from_path(&settings.firmware.metadata)?;

    let machine_addr =
        actor::Machine::new(StateMachine::new(), settings, runtime_settings, firmware).start();
    actix_web::HttpServer::new(move || {
        actix_web::App::new().configure(|cfg| http_api::API::configure(cfg, machine_addr.clone()))
    })
    .bind(listen_socket.clone())
    .unwrap_or_else(|_| panic!("Failed to bind listen socket, {:?}, for HTTP API", listen_socket,))
    .run()
    .await?;

    info!("actix System has stopped");
    Ok(())
}
