// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

macro_rules! create_state_step {
    ($source:ident => $dest:ident) => {
        impl From<State<$source>> for State<$dest> {
            fn from(_from: State<$source>) -> State<$dest> {
                Self($dest {})
            }
        }
    };
    ($source:ident => $dest:ident($field:ident)) => {
        impl From<State<$source>> for State<$dest> {
            fn from(from: State<$source>) -> State<$dest> {
                Self($dest {
                    $field: (from.0).$field,
                })
            }
        }
    };
}

#[cfg(test)]
macro_rules! assert_state {
    ($machine:ident, $state:ident) => {
        assert!(
            if let Ok(StateMachine::$state(_)) = $machine {
                true
            } else {
                false
            },
            "Failed to get to {} state.",
            stringify!($state),
        );
    };
}
