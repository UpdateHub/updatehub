// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
//

use failure::Error;
use states::{Idle, Reboot, State, StateChangeImpl, StateMachine};
use update_package::UpdatePackage;

#[derive(Debug, PartialEq)]
pub struct Install {
    pub update_package: UpdatePackage,
}

create_state_step!(Install => Idle);
create_state_step!(Install => Reboot);

impl StateChangeImpl for State<Install> {
    // FIXME: When adding state-chance hooks, we need to go to Idle if
    // cancelled.
    fn to_next_state(self) -> Result<StateMachine, Error> {
        Ok(StateMachine::Reboot(self.into()))
    }
}
