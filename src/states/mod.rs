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
//!   .---------------.
//!   |               v
//! Idle -> Poll -> Probe -> Download -> Install -> Reboot
//!   ^      ^        '          '          '
//!   '      '        '          '          '
//!   '      `--------'          '          '
//!   `---------------'          '          '
//!   `--------------------------'          '
//!   `-------------------------------------'
//! ```

use failure::Error;
use firmware::Metadata;
use runtime_settings::RuntimeSettings;
use settings::Settings;

#[macro_use]
mod macros;

mod idle;
use self::idle::Idle;

mod poll;
use self::poll::Poll;

mod probe;
use self::probe::Probe;

mod download;
use self::download::Download;

mod install;
use self::install::Install;

pub trait StateChangeImpl {
    fn to_next_state(self) -> Result<StateMachine, Error>;
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

    /// Package UID applied
    applied_package_uid: Option<String>,

    /// State type with specific data and methods.
    state: S,
}

/// The struct representing the state machine.
#[derive(Debug, PartialEq)]
pub enum StateMachine {
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
}

impl StateMachine {
    pub fn new(settings: Settings, runtime_settings: RuntimeSettings, firmware: Metadata) -> Self {
        StateMachine::Idle(State {
            settings,
            runtime_settings,
            firmware,
            applied_package_uid: None,
            state: Idle {},
        })
    }

    pub fn start(self) {
        // FIXME: Handle errors
        self.step().unwrap();
    }

    fn step(self) -> Result<StateMachine, Error> {
        match self {
            StateMachine::Idle(s) => Ok(s.to_next_state()?),
            StateMachine::Poll(s) => Ok(s.to_next_state()?),
            StateMachine::Probe(s) => Ok(s.to_next_state()?),
            StateMachine::Download(s) => Ok(s.to_next_state()?),
            StateMachine::Install(s) => Ok(s.to_next_state()?),
        }
    }
}
