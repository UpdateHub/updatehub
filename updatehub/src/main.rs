// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0
use slog_scope::info;

use structopt::StructOpt;
use updatehub;

#[derive(StructOpt, Debug)]
#[structopt(
    name = "updatehub",
    author = "O.S. Systems Software LTDA. <contact@ossystems.com.br>",
    about = "A generic and safe Firmware Over-The-Air agent."
)]
#[structopt(raw(version = "updatehub::version()"))]
struct Opt {
    /// Increase the verboseness level
    #[structopt(short = "v", long = "verbose", parse(from_occurrences))]
    verbose: usize,
}

fn run() -> Result<(), failure::Error> {
    let opt = Opt::from_args();

    updatehub::logger::init(opt.verbose);
    info!("Starting UpdateHub Agent {}", updatehub::version());

    let settings = updatehub::Settings::load()?;
    updatehub::run(settings)?;

    Ok(())
}

fn main() {
    if let Err(ref e) = run() {
        eprintln!("{}", e);
        e.iter_causes()
            .skip(1)
            .for_each(|e| eprintln!(" caused by: {}\n", e));

        std::process::exit(1);
    }
}
