// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
//

#[macro_use]
extern crate log;
extern crate stderrlog;
extern crate updatehub;

mod cmdline;

fn run() -> updatehub::Result<()> {
    let cmdline = cmdline::CmdLine::parse_args();

    stderrlog::new()
        .quiet(cmdline.quiet)
        .verbosity(if cmdline.debug { 3 } else { 2 })
        .init()?;

    info!(
        "Starting UpdateHub Agent {}",
        updatehub::build_info::version()
    );

    let settings = updatehub::settings::Settings::new().load()?;
    let runtime_settings = updatehub::runtime_settings::RuntimeSettings::new()
        .load(&settings.storage.runtime_settings)?;
    let firmware = updatehub::firmware::Metadata::new(&settings.firmware.metadata_path)?;

    updatehub::states::StateMachine::new(settings, runtime_settings, firmware).run();

    Ok(())
}

fn main() {
    if let Err(ref e) = run() {
        error!("{}", e);
        e.iter_causes()
            .skip(1)
            .for_each(|e| error!(" caused by: {}\n", e));

        std::process::exit(1);
    }
}
