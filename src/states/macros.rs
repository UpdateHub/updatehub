// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
// 

macro_rules! create_state_step {
    ($source:ident => $dest:ident) => {
        impl From<State<$source>> for State<$dest> {
            fn from(from: State<$source>) -> State<$dest> {
                State { settings: from.settings,
                        runtime_settings: from.runtime_settings,
                        firmware: from.firmware,
                        applied_package_uid: None,
                        state: $dest {}, }
            }
        }
    }
}

#[cfg(test)]
macro_rules! assert_state {
    ($machine:ident, $state:ident) => {
        assert!(if let StateMachine::$state(_) = $machine {
            true
        } else {
            false
        });
    }
}
