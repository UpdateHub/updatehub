// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: GPL-2.0
// 

#![cfg_attr(feature = "clippy", feature(plugin))]
#![cfg_attr(feature = "clippy", plugin(clippy))]

extern crate core;

#[macro_use]
extern crate log;
extern crate stderrlog;

extern crate serde;
#[macro_use]
extern crate serde_derive;
extern crate serde_ini;

extern crate checked_command;
extern crate cmdline_words_parser;
extern crate parse_duration;

extern crate walkdir;

#[cfg(test)]
extern crate mktemp;

mod build_info;

mod serde_helpers;

mod settings;
mod runtime_settings;
mod cmdline;

mod process;
mod firmware;

use cmdline::CmdLine;
use firmware::Metadata as FirmwareMetadata;
use runtime_settings::RuntimeSettings;
use settings::Settings;

fn main() {
    let cmdline = CmdLine::parse_args();

    stderrlog::new().quiet(cmdline.quiet)
                    .verbosity(if cmdline.debug { 3 } else { 2 })
                    .init()
                    .expect("Failed to initialize the logger.");

    info!("Starting UpdateHub Agent {}", build_info::version());

    let settings = Settings::new().load().expect("Failed to load settings.");
    let runtime_settings = RuntimeSettings::new().load(&settings.storage.runtime_settings)
                                                 .expect("Failed to load runtime settings.");
    let firmware_metadata =
        FirmwareMetadata::new(&settings.firmware.metadata_path).expect("Failed to load the firmware metadata.");
}
