// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

#[macro_use]
mod macros;
mod direct_download;
mod download;
mod entry_point;
mod error;
pub(crate) mod install;
pub(crate) mod machine;
mod park;
mod poll;
mod prepare_local_install;
mod probe;
mod reboot;
mod validation;

#[cfg(test)]
mod tests;

use self::{
    direct_download::DirectDownload, download::Download, entry_point::EntryPoint, error::Error,
    install::Install, park::Park, poll::Poll, prepare_local_install::PrepareLocalInstall,
    probe::Probe, reboot::Reboot, validation::Validation,
};
use crate::{
    firmware::{self, Metadata, Transition},
    http_api,
    runtime_settings::RuntimeSettings,
    settings::Settings,
};
use async_trait::async_trait;
use derive_more::{Display, Error, From};
use slog_scope::{error, info, trace, warn};
use std::path::Path;

pub type Result<T> = std::result::Result<T, TransitionError>;

#[derive(Debug, Display, Error, From)]
pub enum TransitionError {
    #[display(fmt = "some objects are not ready for use")]
    SomeObjectsAreNotReady,
    #[display(fmt = "signature not found")]
    SignatureNotFound,
    #[display(fmt = "channel communication as failed")]
    CommunicationFailed,

    Firmware(crate::firmware::Error),
    Installation(crate::object::Error),
    RuntimeSettings(crate::runtime_settings::Error),
    UpdatePackage(crate::update_package::Error),
    Client(cloud::Error),
    Uncompress(compress_tools::Error),
    SerdeJson(serde_json::error::Error),
    Io(std::io::Error),
    NonUtf8(std::str::Utf8Error),
    Process(easy_process::Error),
}

#[async_trait(?Send)]
trait StateChangeImpl {
    async fn handle(
        self,
        context: &mut machine::Context,
    ) -> Result<(State, machine::StepTransition)>;

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

#[async_trait(?Send)]
trait CallbackReporter: Sized + StateChangeImpl {
    async fn handle_on_transition_cancel(&self, _context: &mut machine::Context) -> Result<()> {
        Ok(())
    }

    async fn handle_on_error(&self, _context: &mut machine::Context) -> Result<()> {
        Ok(())
    }

    async fn handle_with_callback(
        self,
        context: &mut machine::Context,
    ) -> Result<(State, machine::StepTransition)> {
        let transition =
            firmware::state_change_callback(&context.settings.firmware.metadata, self.name());

        match transition {
            Ok(Transition::Continue) => return self.handle(context).await,
            Ok(Transition::Cancel) => {
                info!(
                    "canceling transition to '{}' due to state change callback request",
                    self.name()
                );

                self.handle_on_transition_cancel(context).await
                    .unwrap_or_else(|e| error!("failed calling specialized handler for canceling \
                                               transition to '{}' as state change callback has failed with: {}",
                                               self.name(),
                                               e));
            }
            Err(e) => {
                error!(
                    "canceling transition to '{}' as state change callback has failed with: {}",
                    self.name(),
                    e
                );

                self.handle_on_error(context).await
                    .unwrap_or_else(|e| error!("failed calling specialized handler for transition \
                                                error to '{}' as state change callback has failed with: {}",
                                               self.name(),
                                               e));
            }
        }

        Ok((State::EntryPoint(EntryPoint {}), machine::StepTransition::Immediate))
    }
}

#[async_trait(?Send)]
trait ProgressReporter: CallbackReporter {
    fn package_uid(&self) -> String;
    fn report_enter_state_name(&self) -> &'static str;
    fn report_leave_state_name(&self) -> &'static str;

    async fn handle_and_report_progress(
        self,
        context: &mut machine::Context,
    ) -> Result<(State, machine::StepTransition)> {
        let server = context.server_address().to_owned();
        let firmware = &context.firmware.clone();
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
        match self.handle(context).await {
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

    async fn handle_with_callback_and_report_progress(
        self,
        context: &mut machine::Context,
    ) -> Result<(State, machine::StepTransition)> {
        let transition =
            firmware::state_change_callback(&context.settings.firmware.metadata, self.name())?;

        match transition {
            Transition::Continue => Ok(self.handle_and_report_progress(context).await?),
            Transition::Cancel => {
                info!(
                    "canceling transition to '{}' due to state change callback request",
                    self.name()
                );
                Ok((State::EntryPoint(EntryPoint {}), machine::StepTransition::Immediate))
            }
        }
    }
}

#[derive(Debug)]
enum State {
    Park(Park),
    EntryPoint(EntryPoint),
    Poll(Poll),
    Probe(Probe),
    Validation(Validation),
    Download(Download),
    Install(Install),
    Reboot(Reboot),
    DirectDownload(DirectDownload),
    PrepareLocalInstall(PrepareLocalInstall),
    Error(Error),
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

                    // In case we are booting from an UpdateHub v1 update and
                    // the validation has failed, we need to restore the
                    // original content of the file to not break the rollback
                    // procedure when rebooting.
                    #[cfg(feature = "v1-parsing")]
                    runtime_settings.restore_v1_content()?;

                    easy_process::run("reboot")?;

                    // Ensure we detect the rollback in next boot.
                    return Ok(());
                }
                Transition::Continue => firmware::installation_set::validate()?,
            }
        } else {
            warn!("confirming active installation as update has been rollback");
            firmware::installation_set::validate()?;
        }

        runtime_settings.reset_installation_settings()?;
    }
    Ok(())
}

#[async_trait(?Send)]
impl StateChangeImpl for State {
    async fn handle(self, st: &mut machine::Context) -> Result<(State, machine::StepTransition)> {
        trace!("starting to handle '{}' state", self.name());
        self.move_to_next_state(st).await
    }

    fn name(&self) -> &'static str {
        self.inner_state().name()
    }

    fn is_handling_download(&self) -> bool {
        self.inner_state().is_handling_download()
    }

    fn is_preemptive_state(&self) -> bool {
        self.inner_state().is_preemptive_state()
    }
}

impl State {
    fn new() -> Self {
        State::EntryPoint(EntryPoint {})
    }

    async fn move_to_next_state(
        self,
        context: &mut machine::Context,
    ) -> Result<(Self, machine::StepTransition)> {
        match self {
            State::Park(s) => s.handle(context).await,
            State::EntryPoint(s) => s.handle(context).await,
            State::Poll(s) => s.handle(context).await,
            State::Probe(s) => s.handle_with_callback(context).await,
            State::Validation(s) => s.handle(context).await,
            State::DirectDownload(s) => s.handle(context).await,
            State::PrepareLocalInstall(s) => s.handle_with_callback(context).await,
            State::Error(s) => s.handle_with_callback(context).await,
            State::Download(s) => s.handle_with_callback_and_report_progress(context).await,
            State::Install(s) => s.handle_with_callback_and_report_progress(context).await,
            State::Reboot(s) => s.handle_with_callback_and_report_progress(context).await,
        }
    }

    fn inner_state(&self) -> &dyn StateChangeImpl {
        match self {
            State::Error(s) => s,
            State::Park(s) => s,
            State::EntryPoint(s) => s,
            State::Poll(s) => s,
            State::Probe(s) => s,
            State::Validation(s) => s,
            State::DirectDownload(s) => s,
            State::PrepareLocalInstall(s) => s,
            State::Download(s) => s,
            State::Install(s) => s,
            State::Reboot(s) => s,
        }
    }
}

/// Runs the state machine up to completion handling all processing
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
/// # use std::path::PathBuf;
/// # async fn run() -> Result<(), updatehub::Error> {
/// use updatehub;
/// use std::path::PathBuf;
///
/// updatehub::logger::init(slog::Level::Info);
/// updatehub::run(&PathBuf::from("/etc/updatehub.conf")).await?;
/// # Ok(())
/// # }
/// ```
pub async fn run(settings: &Path) -> crate::Result<()> {
    crate::logger::start_memory_logging();
    let settings = Settings::load(settings)?;
    let listen_socket = settings.network.listen_socket.clone();
    let mut runtime_settings = RuntimeSettings::load(&settings.storage.runtime_settings)?;
    if !settings.storage.read_only {
        runtime_settings.enable_persistency();
    }
    let firmware = Metadata::from_path(&settings.firmware.metadata)?;

    if let Err(e) = handle_startup_callbacks(&settings, &mut runtime_settings) {
        error!("Failed to handle startup callbacks: {}", e);
    }

    let machine = machine::StateMachine::new(State::new(), settings, runtime_settings, firmware);
    let addr = machine.address();

    // Use a local spawn since running features are !Send
    tokio::task::spawn_local(machine.start());

    // FIXME: handle failiure to parse the listen socket
    http_api::Api::server(addr)
        .run(listen_socket.replace("localhost", "127.0.0.1").parse::<std::net::SocketAddr>()?)
        .await;

    info!("Server has gracefully stopped");
    Ok(())
}
