// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
//

extern crate app;
use self::app::{App, Opt};

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
