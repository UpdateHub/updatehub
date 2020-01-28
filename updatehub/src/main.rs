// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0
use slog_scope::info;

use structopt::StructOpt;

#[derive(StructOpt, Debug)]
#[structopt(
    no_version,
    name = "updatehub",
    author = "O.S. Systems Software LTDA. <contact@ossystems.com.br>",
    about = "A generic and safe Firmware Over-The-Air agent.",
    version = updatehub::version()
)]
struct Opt {
    /// Increase the verboseness level
    #[structopt(short = "v", long = "verbose", parse(from_occurrences))]
    verbose: usize,
}

async fn run() -> updatehub::Result<()> {
    let opt = Opt::from_args();

    updatehub::logger::init(opt.verbose);
    info!("Starting UpdateHub Agent {}", updatehub::version());

    let settings = updatehub::Settings::load()?;
    updatehub::run(settings).await?;

    Ok(())
}

#[actix_rt::main]
async fn main() {
    if let Err(ref e) = run().await {
        eprintln!("{}", e);

        std::process::exit(1);
    }
}
