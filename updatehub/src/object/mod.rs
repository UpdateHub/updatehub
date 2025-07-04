// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

#[macro_use]
mod macros;

pub(crate) mod info;
pub(crate) mod installer;

pub(crate) use self::{info::Info, installer::Installer};
use derive_more::{Display, Error, From};

pub type Result<T> = std::result::Result<T, Error>;

#[derive(Debug, Display, Error, From)]
pub enum Error {
    #[display(fmt = "invalid path formed")]
    InvalidPath,
    #[display(fmt = "'fw_setenv' does not support the --script command line option")]
    FwSetEnvNoScriptOption,
    #[display(fmt = "unsupported object model")]
    Unsupported,

    Utils(crate::utils::Error),
    Firmware(crate::firmware::Error),

    #[display(fmt = "invalid target type {_0:?}")]
    InvalidTargetType(#[error(not(source))] pkg_schema::definitions::TargetType),
    Io(std::io::Error),
    Process(easy_process::Error),
    Uncompress(compress_tools::Error),
}
