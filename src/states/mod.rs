// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
//

//! Controls the state machine of the system
//!
//! It supports following states, and transitions, as shown in the
//! below diagram:
//!
//! ```text
//!           .--------------.
//!           |              v
//! Park <- Idle -> Poll -> Probe -> Download -> Install -> Reboot
//!           ^      ^        '          '          '
//!           '      '        '          '          '
//!           '      `--------'          '          '
//!           `---------------'          '          '
//!           `--------------------------'          '
//!           `-------------------------------------'
//! ```

#[macro_use]
mod macros;
mod download;
mod idle;
mod install;
mod park;
mod poll;
mod probe;
mod reboot;
mod transition;

use Result;

pub use self::{
    download::Download, idle::Idle, install::Install, park::Park, poll::Poll, probe::Probe,
    reboot::Reboot,
};

use firmware::Metadata;
use runtime_settings::RuntimeSettings;
use settings::Settings;

pub trait StateChangeImpl {
    fn handle(self) -> Result<StateMachine>;
}

pub trait TransitionCallback: Into<State<Idle>> {
    fn callback_state_name(&self) -> &'static str;
}

/// Holds the `State` type and common data, which is available for
/// every state transition.
#[derive(Debug, PartialEq)]
pub struct State<S>
where
    State<S>: StateChangeImpl,
{
    /// System settings.
    settings: Settings,

    /// Runtime settings.
    runtime_settings: RuntimeSettings,

    /// Firmware metadata.
    firmware: Metadata,

    /// State type with specific data and methods.
    state: S,
}

/// The struct representing the state machine.
#[derive(Debug, PartialEq)]
pub enum StateMachine {
    /// Park state
    Park(State<Park>),

    /// Idle state
    Idle(State<Idle>),

    /// Poll state
    Poll(State<Poll>),

    /// Probe state
    Probe(State<Probe>),

    /// Download state
    Download(State<Download>),

    /// Install state
    Install(State<Install>),

    /// Reboot state
    Reboot(State<Reboot>),
}

impl<S> State<S>
where
    State<S>: TransitionCallback + StateChangeImpl,
{
    fn handle_with_callback(self) -> Result<StateMachine> {
        use states::transition::{state_change_callback, Transition};

        let transition = state_change_callback(
            &self.settings.firmware.metadata_path,
            self.callback_state_name(),
        )?;

        match transition {
            Transition::Continue => Ok(self.handle()?),
            Transition::Cancel => Ok(StateMachine::Idle(self.into())),
        }
    }
}

impl StateMachine {
    pub fn new(settings: Settings, runtime_settings: RuntimeSettings, firmware: Metadata) -> Self {
        StateMachine::Idle(State {
            settings,
            runtime_settings,
            firmware,
            state: Idle {},
        })
    }

    pub fn run(self) {
        self.step()
    }

    fn step(self) {
        match self.move_to_next_state() {
            Ok(StateMachine::Park(_)) => {
                debug!("Parking state machine.");
                return;
            }
            Ok(s) => s.run(),
            Err(e) => panic!("{}", e),
        }
    }

    fn move_to_next_state(self) -> Result<Self> {
        match self {
            StateMachine::Park(s) => Ok(s.handle()?),
            StateMachine::Idle(s) => Ok(s.handle()?),
            StateMachine::Poll(s) => Ok(s.handle()?),
            StateMachine::Probe(s) => Ok(s.handle()?),
            StateMachine::Download(s) => Ok(s.handle_with_callback()?),
            StateMachine::Install(s) => Ok(s.handle_with_callback()?),
            StateMachine::Reboot(s) => Ok(s.handle_with_callback()?),
        }
    }
}
