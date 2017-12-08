/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contact@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

#[macro_use]
extern crate log;
extern crate stderrlog;

#[macro_use]
extern crate serde_derive;
extern crate serde_ini;
extern crate serde;

extern crate parse_duration;

mod de_helpers;

mod settings;
mod cmdline;

use cmdline::CmdLine;
use settings::Settings;

fn main() {
    let cmdline = CmdLine::parse_args();

    stderrlog::new()
        .quiet(cmdline.quiet)
        .verbosity(if cmdline.debug { 3 } else { 1 })
        .verbosity(4)
        .init()
        .expect("Failed to initialize the logger.");

    let settings = Settings::new().load().expect("Failed to load settings.");
}
