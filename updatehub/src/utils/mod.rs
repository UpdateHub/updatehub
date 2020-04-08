// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

pub(crate) mod definitions;
pub(crate) mod fs;
pub(crate) mod io;
pub(crate) mod mtd;

use thiserror::Error;

pub type Result<T> = std::result::Result<T, Error>;

#[derive(Debug, Error)]
pub enum Error {
    #[error("Io: {0}")]
    Io(#[from] std::io::Error),

    #[error("Nix error: {0}")]
    Nix(#[from] nix::Error),

    #[error("Uncompress error: {0}")]
    Uncompress(#[from] compress_tools::Error),

    #[error("Process error: {0}")]
    Process(#[from] easy_process::Error),

    #[error("Strip prefix error: {0}")]
    StripPrefix(#[from] std::path::StripPrefixError),

    #[error("Target device does not exists")]
    DeviceDoesNotExist,

    #[error("User doesn't have write permission on target device: {0}")]
    MissingWritePermission(std::path::PathBuf),

    #[error("'{0}' not found on PATH")]
    ExecutableNotInPath(String),

    #[error("Unable to find Ubi Volume: {0}")]
    NoUbiVolume(String),

    #[error("Unable to find match for mtd device: {0}")]
    NoMtdDevice(String),
}

/// Encode a bytes stream in hex
pub(crate) fn hex_encode(data: &[u8]) -> String {
    data.iter().map(|c| format!("{:02x}", c)).collect()
}

/// Get sha256sum hash from a byte stream
pub(crate) fn sha256sum(data: &[u8]) -> String {
    hex_encode(&openssl::sha::sha256(data))
}
