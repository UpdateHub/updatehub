// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use derive_more::{Display, From};

pub(crate) mod definitions;
pub(crate) mod fs;
pub(crate) mod io;
pub(crate) mod mtd;

pub type Result<T> = std::result::Result<T, Error>;

#[derive(Debug, Display, From)]
pub enum Error {
    #[display(fmt = "Io: {}", _0)]
    Io(std::io::Error),
    #[display(fmt = "Nix error: {}", _0)]
    Nix(nix::Error),
    #[display(fmt = "Uncompress error")]
    Uncompress,
    #[display(fmt = "Process error: {}", _0)]
    Process(easy_process::Error),
    #[display(fmt = "Strip prefix error: {}", _0)]
    StripPrefix(std::path::StripPrefixError),

    #[display(fmt = "Target device does not exists")]
    DeviceDoesNotExist,
    #[display(fmt = "User doesn't have write permission on target device: {:?}", _0)]
    #[from(ignore)]
    MissingWritePermission(std::path::PathBuf),
    #[display(fmt = "Unknown file type")]
    UknownFileType,
    #[display(fmt = "{} is not a valid archive type", _0)]
    #[from(ignore)]
    InvalidFileType(String),
    #[display(fmt = "'{}' not found on PATH", _0)]
    #[from(ignore)]
    ExecutableNotInPath(String),
    #[display(fmt = "Unable to find Ubi Volume: {}", _0)]
    #[from(ignore)]
    NoUbiVolume(String),
    #[display(fmt = "Unable to find match for mtd device: {}", _0)]
    #[from(ignore)]
    NoMtdDevice(String),
}
