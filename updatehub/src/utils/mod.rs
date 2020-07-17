// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

pub(crate) mod definitions;
pub(crate) mod fs;
pub(crate) mod io;
pub(crate) mod mtd;

use derive_more::{Display, Error, From};

pub type Result<T> = std::result::Result<T, Error>;

#[derive(Debug, Display, Error, From)]
pub enum Error {
    Io(std::io::Error),
    Nix(nix::Error),
    Uncompress(compress_tools::Error),
    Process(easy_process::Error),
    StripPrefix(std::path::StripPrefixError),

    #[display("Target device does not exists")]
    DeviceDoesNotExist,

    #[display(fmt = "User doesn't have write permission on target device: {:?}", _0)]
    MissingWritePermission(#[error(not(source))] std::path::PathBuf),

    #[display("Not enough storage space for installation")]
    NotEnoughSpace,

    #[display(fmt = "'{}' not found on PATH", _0)]
    #[from(ignore)]
    ExecutableNotInPath(#[error(not(source))] String),
    #[display(fmt = "Unable to find Ubi Volume: {}" _0)]
    #[from(ignore)]
    NoUbiVolume(#[error(not(source))] String),
    #[display(fmt = "Unable to find match for mtd device: {}", _0)]
    #[from(ignore)]
    NoMtdDevice(#[error(not(source))] String),
}

/// Encode a bytes stream in hex
pub(crate) fn hex_encode(data: &[u8]) -> String {
    data.iter().map(|c| format!("{:02x}", c)).collect()
}

/// Get sha256sum hash from a byte stream
pub(crate) fn sha256sum(data: &[u8]) -> String {
    hex_encode(&openssl::sha::sha256(data))
}
