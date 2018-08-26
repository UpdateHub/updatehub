// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
//

#[macro_use]
extern crate log;
extern crate app;
extern crate stderrlog;
extern crate updatehub;

use app::{App, Opt};
use std::env;

impl CmdLine {
    pub fn parse_args() -> Self {
        let mut config = CmdLine::default();

        let helper = {
            App::new("updatehub")
                .version(env::var("CARGO_PKG_VERSION").unwrap_or_else(|_| "Unknown".to_string()))
                .author("O.S. Systems Software LTDA.", "contact@ossystems.com.br")
                .desc("A generic and safe Firmware Over-The-Air agent.")
                .opt(
                    Opt::new("debug", &mut config.debug)
                        .short('d')
                        .long("debug")
                        .help("Enable debug messages"),
                ).opt(
                    Opt::new("quiet", &mut config.quiet)
                        .short('q')
                        .long("quiet")
                        .help("Disable informative message"),
                ).parse_args()
        };

        config
            .check()
            .map_err(|e| helper.help_err_exit(e, 1))
            .unwrap() // help_err_exit already exits, so unwrap is safe here!
    }

    fn check(self) -> Result<Self, String> {
        if self.debug && self.quiet {
            return Err("You cannot enable 'quiet' and 'debug' at same time.".to_string());
        }

        Ok(self)
    }
}

#[derive(Default)]
pub struct CmdLine {
    pub debug: bool,
    pub quiet: bool,
}

fn run() -> updatehub::Result<()> {
    let cmdline = CmdLine::parse_args();

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
