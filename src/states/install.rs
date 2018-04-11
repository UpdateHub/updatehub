// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
//

use states::{State, StateChangeImpl, StateMachine};

use states::idle::Idle;
use update_package::UpdatePackage;

#[derive(Debug, PartialEq)]
pub struct Install {
    pub update_package: UpdatePackage,
}

create_state_step!(Install => Idle);

impl StateChangeImpl for State<Install> {
    fn to_next_state(self) -> StateMachine {
        StateMachine::Idle(self.into())
    }
}
