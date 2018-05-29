// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
//

use failure::Error;
use states::{Idle, State, StateChangeImpl, StateMachine};
use update_package::UpdatePackage;

#[derive(Debug, PartialEq)]
pub struct Install {
    pub update_package: UpdatePackage,
}

create_state_step!(Install => Idle);

impl StateChangeImpl for State<Install> {
    fn to_next_state(self) -> Result<StateMachine, Error> {
        Ok(StateMachine::Idle(self.into()))
    }
}
