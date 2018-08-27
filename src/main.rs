// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
//

#[macro_use]
extern crate log;
extern crate stderrlog;
#[macro_use]
extern crate structopt;
extern crate updatehub;

use structopt::StructOpt;

#[derive(StructOpt, Debug)]
#[structopt(
    name = "updatehub",
    author = "O.S. Systems Software LTDA. <contact@ossystems.com.br>",
    about = "A generic and safe Firmware Over-The-Air agent."
)]
#[structopt(raw(version = "updatehub::build_info::version()"))]
struct Opt {
    /// Increase the verboseness level
    #[structopt(short = "v", long = "verbose", parse(from_occurrences))]
    verbose: u8,
}

fn run() -> updatehub::Result<()> {
    let opt = Opt::from_args();

    stderrlog::new()
        .verbosity(opt.verbose as usize + 1)
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
