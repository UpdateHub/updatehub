// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: GPL-2.0-only
// 

#![cfg_attr(feature = "clippy", feature(plugin))]
#![cfg_attr(feature = "clippy", plugin(clippy))]

extern crate updatehub;

#[macro_use]
extern crate log;
extern crate stderrlog;

use updatehub::build_info;
use updatehub::firmware::Metadata as FirmwareMetadata;
use updatehub::runtime_settings::RuntimeSettings;
use updatehub::settings::Settings;

mod cmdline;
use cmdline::CmdLine;

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
