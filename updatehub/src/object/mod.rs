// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

#[macro_use]
mod macros;

pub(crate) mod info;
pub(crate) mod installer;

pub(crate) use self::{info::Info, installer::Installer};

use thiserror::Error;

pub type Result<T> = std::result::Result<T, Error>;

#[derive(Debug, Error)]
pub enum Error {
    #[error("Invalid path formed")]
    InvalidPath,

    #[error("Unsupported target type: {0:?}")]
    InvalidTargetType(pkg_schema::definitions::TargetType),

    #[error("Utils error: {0}")]
    Utils(#[from] crate::utils::Error),

    #[error("Io error: {0}")]
    Io(#[from] std::io::Error),

    #[error("Process error: {0}")]
    Process(#[from] easy_process::Error),
}
