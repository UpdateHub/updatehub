// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
//

extern crate updatehub;

extern crate failure;
#[macro_use]
extern crate log;
extern crate stderrlog;

use updatehub::build_info;
use updatehub::firmware::Metadata;
use updatehub::runtime_settings::RuntimeSettings;
use updatehub::settings::Settings;
use updatehub::states::StateMachine;
use updatehub::Error;

mod cmdline;
use cmdline::CmdLine;

fn run() -> Result<(), Error> {
    let cmdline = CmdLine::parse_args();

    stderrlog::new()
        .quiet(cmdline.quiet)
        .verbosity(if cmdline.debug { 3 } else { 2 })
        .init()?;

    info!("Starting UpdateHub Agent {}", build_info::version());

    let settings = Settings::new().load()?;
    let runtime_settings = RuntimeSettings::new().load(&settings.storage.runtime_settings)?;
    let firmware = Metadata::new(&settings.firmware.metadata_path)?;

    StateMachine::new(settings, runtime_settings, firmware).start();

    Ok(())
}

fn main() {
    if let Err(ref e) = run() {
        error!("{}", e);
        e.causes()
            .skip(1)
            .for_each(|e| error!(" caused by: {}\n", e));

        std::process::exit(1);
    }
}
