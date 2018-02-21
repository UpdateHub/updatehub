// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
// 

use states::{State, StateChangeImpl, StateMachine};

use states::idle::Idle;
use update_package::UpdatePackage;

#[derive(Debug, PartialEq)]
pub struct Download {
    pub update_package: UpdatePackage,
}

create_state_step!(Download => Idle);

impl StateChangeImpl for State<Download> {
    fn to_next_state(self) -> StateMachine {
        println!("{:?}", self.state.update_package);

        StateMachine::Idle(self.into())
    }
}
