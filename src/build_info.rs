//
// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: GPL-2.0
//

const VERSION: &str = env!("VERSION");

pub fn version() -> &'static str {
    VERSION
}
