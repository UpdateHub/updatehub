// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

#[macro_use]
mod macros;
pub(crate) mod actor;
mod direct_download;
mod download;
mod entry_point;
mod error;
pub(crate) mod install;
mod park;
mod poll;
mod prepare_download;
mod prepare_local_install;
mod probe;
mod reboot;
mod validation;

#[cfg(test)]
mod tests;

use self::{
    direct_download::DirectDownload, download::Download, entry_point::EntryPoint, error::Error,
    install::Install, park::Park, poll::Poll, prepare_download::PrepareDownload,
    prepare_local_install::PrepareLocalInstall, probe::Probe, reboot::Reboot,
    validation::Validation,
};
use crate::{
    firmware::{self, Metadata, Transition},
    http_api,
    runtime_settings::RuntimeSettings,
    settings::Settings,
};
use async_trait::async_trait;
use slog_scope::{error, info, warn};
use thiserror::Error;

pub type Result<T> = std::result::Result<T, TransitionError>;

#[derive(Debug, Error)]
pub enum TransitionError {
    #[error("not all objects are ready for use")]
    ObjectsNotReady,

    #[error("signature not found")]
    SignatureNotFound,

    #[error(transparent)]
    Firmware(#[from] crate::firmware::Error),

    #[error(transparent)]
    Installation(#[from] crate::object::Error),

    #[error(transparent)]
    RuntimeSettings(#[from] crate::runtime_settings::Error),

    #[error(transparent)]
    UpdatePackage(#[from] crate::update_package::Error),

    #[error(transparent)]
    Client(#[from] cloud::Error),

    #[error(transparent)]
    Uncompress(#[from] compress_tools::Error),

    #[error("serde error: {0}")]
    SerdeJson(#[from] serde_json::error::Error),

    #[error("update package error: {0}")]
    Io(#[from] std::io::Error),

    #[error("non Utf8 error: {0}")]
    NonUtf8(#[from] std::string::FromUtf8Error),

    #[error("mailbox error: {0}")]
    ActixMailbox(#[from] actix::MailboxError),

    #[error("process error: {0}")]
    Process(#[from] easy_process::Error),
}

#[async_trait(?Send)]
trait StateChangeImpl {
    async fn handle(
        self,
        shared_state: &mut actor::SharedState,
    ) -> Result<(StateMachine, actor::StepTransition)>;

    fn name(&self) -> &'static str;

    /// All states downloading large files should overwrite this to return true.
    /// That way, a external request to abort download can be heeded.
    fn is_handling_download(&self) -> bool {
        false
    }

    /// A preemptive state is a state whose transition can be yield
    /// to handle a user's request. Any preemptive state should overwrite
    /// this method to return true.
    fn is_preemptive_state(&self) -> bool {
        false
    }
}

trait TransitionCallback: StateChangeImpl + Into<State<EntryPoint>> {}

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
    EntryPoint(State<EntryPoint>),
    Poll(State<Poll>),
    Probe(State<Probe>),
    Validation(State<Validation>),
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
        let transition =
            firmware::state_change_callback(&shared_state.settings.firmware.metadata, self.name())?;

        match transition {
            Transition::Continue => Ok(self.handle_and_report_progress(shared_state).await?),
            Transition::Cancel => {
                Ok((StateMachine::EntryPoint(self.into()), actor::StepTransition::Immediate))
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
        let api = crate::CloudClient::new(&server);

        let report = |state, previous_state, error_message, current_log| {
            api.report(
                state,
                firmware.as_cloud_metadata(),
                package_uid,
                previous_state,
                error_message,
                current_log,
            )
        };

        if let Err(e) = report(enter_state, None, None, None).await {
            warn!("report failed: {}", e);
        }
        match self.handle(shared_state).await {
            Ok((state, trans)) => {
                if let Err(e) = report(leave_state, None, None, None).await {
                    warn!("report failed: {}", e);
                };
                Ok((state, trans))
            }
            Err(e) => {
                if let Err(e) = report(
                    "error",
                    Some(enter_state),
                    Some(e.to_string()),
                    Some(crate::logger::get_memory_log()),
                )
                .await
                {
                    warn!("report failed: {}", e);
                }
                Err(e)
            }
        }
    }
}

fn handle_startup_callbacks(
    settings: &Settings,
    runtime_settings: &mut RuntimeSettings,
) -> crate::Result<()> {
    if let Some(expected_set) = runtime_settings.update.upgrade_to_installation {
        info!("booting from a recent installation");
        if expected_set == firmware::installation_set::active()?.0 {
            match firmware::validate_callback(&settings.firmware.metadata)? {
                Transition::Cancel => {
                    warn!("validate callback has failed");
                    firmware::installation_set::swap_active()?;
                    warn!("swapped active installation set and running rollback");
                    firmware::rollback_callback(&settings.firmware.metadata)?;
                    runtime_settings.reset_installation_settings()?;
                    easy_process::run("reboot")?;
                }
                Transition::Continue => firmware::installation_set::validate()?,
            }
        }
        runtime_settings.reset_installation_settings()?;
    }
    Ok(())
}

impl StateMachine {
    fn new() -> Self {
        StateMachine::EntryPoint(State(EntryPoint {}))
    }

    async fn move_to_next_state(
        self,
        shared_state: &mut actor::SharedState,
    ) -> Result<(Self, actor::StepTransition)> {
        match self {
            StateMachine::Error(s) => s.handle(shared_state).await,
            StateMachine::Park(s) => s.handle(shared_state).await,
            StateMachine::EntryPoint(s) => s.handle(shared_state).await,
            StateMachine::Poll(s) => s.handle(shared_state).await,
            StateMachine::Probe(s) => s.handle(shared_state).await,
            StateMachine::Validation(s) => s.handle(shared_state).await,
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

    fn for_current_state<F, A>(&self, f: F) -> A
    where
        F: Fn(&dyn StateChangeImpl) -> A,
    {
        match self {
            StateMachine::Error(s) => f(s),
            StateMachine::Park(s) => f(s),
            StateMachine::EntryPoint(s) => f(s),
            StateMachine::Poll(s) => f(s),
            StateMachine::Probe(s) => f(s),
            StateMachine::Validation(s) => f(s),
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
///             .-------------------.
///             |                   v
/// Park <- EntryPoint -> Poll -> Probe -> Download -> Install -> Reboot
///             ^          ^        '          '          '
///             '          '        '          '          '
///             '          `--------'          '          '
///             `-------------------'          '          '
///             `------------------------------'          '
///             `-----------------------------------------'
/// ```
///
/// # Example
/// ```no_run
/// # extern crate updatehub;
/// # async fn run() -> Result<(), updatehub::Error> {
/// use updatehub;
/// use std::path::PathBuf;
///
/// updatehub::logger::init(slog::Level::Info);
/// let settings = updatehub::Settings::load(&PathBuf::from("/etc/updatehub.conf"))?;
/// updatehub::run(settings).await?;
/// # Ok(())
/// # }
/// ```
pub async fn run(settings: Settings) -> crate::Result<()> {
    crate::logger::start_memory_logging();
    let listen_socket = settings.network.listen_socket.clone();
    let mut runtime_settings = RuntimeSettings::load(&settings.storage.runtime_settings)?;
    if !settings.storage.read_only {
        runtime_settings.enable_persistency();
    }
    let firmware = Metadata::from_path(&settings.firmware.metadata)?;

    if let Err(e) = handle_startup_callbacks(&settings, &mut runtime_settings) {
        error!("Failed to handle startup callbacks: {}", e);
    }

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
