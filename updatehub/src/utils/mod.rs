// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

pub(crate) mod definitions;
pub(crate) mod delta;
pub(crate) mod fs;
pub(crate) mod io;
pub(crate) mod log;
pub(crate) mod mtd;

#[cfg(feature = "v1-parsing")]
pub(crate) mod deserialize;

use derive_more::{Display, Error, From};
use std::fmt::Write;

pub type Result<T> = std::result::Result<T, Error>;

#[derive(Debug, Display, Error, From)]
pub enum Error {
    Io(std::io::Error),
    Nix(nix::Error),
    Uncompress(compress_tools::Error),
    Process(easy_process::Error),
    StripPrefix(std::path::StripPrefixError),

    #[display("{_0:?} target device does not exists")]
    #[from(ignore)]
    DeviceDoesNotExist(#[error(not(source))] std::path::PathBuf),

    #[display("user doesn't have write permission on target device: {_0:?}")]
    #[from(ignore)]
    MissingWritePermission(#[error(not(source))] std::path::PathBuf),

    #[display(
        "target device {device:?} is in use, it holds the filesystem mounted at {mount_point:?}"
    )]
    #[from(ignore)]
    DeviceInUse {
        device: std::path::PathBuf,
        mount_point: std::path::PathBuf,
    },

    #[display(
        "{available} is not enough storage space for installation, at least {required} is required"
    )]
    #[from(ignore)]
    NotEnoughSpace {
        available: u64,
        required: u64,
    },

    #[display("'{_0}' not found on PATH")]
    #[from(ignore)]
    ExecutableNotInPath(#[error(not(source))] String),
    #[display("unable to find Ubi Volume: {_0}")]
    #[from(ignore)]
    NoUbiVolume(#[error(not(source))] String),
    #[display("unable to find match for mtd device: {_0}")]
    #[from(ignore)]
    NoMtdDevice(#[error(not(source))] String),

    #[display("bita operation failed due to io error: {_0}")]
    BitaArchiveIO(bitar::ArchiveError<std::io::Error>),
    #[display("bita operation failed due to remote read error: {_0}")]
    BitaArchiveRemote(bitar::ArchiveError<bitar::archive_reader::HttpReaderError>),
    #[display("bita operation failed due to remote read error: {_0}")]
    BitaRemote(bitar::archive_reader::HttpReaderError),
    #[display("bita operation failed due to compression error: {_0}")]
    BitaCompression(bitar::CompressionError),
    #[display("bita operation failed due to hash sum mismatch error: {_0}")]
    BitaHashSum(Box<bitar::HashSumMismatchError>),
    #[display("bita operation failed due to invalid url: {_0}")]
    BitaUrl(url::ParseError),
}

/// Encode a bytes stream in hex
#[inline]
pub(crate) fn hex_encode(data: &[u8]) -> String {
    data.iter().fold(String::new(), |mut output, c| {
        let _ = write!(output, "{c:02x}");

        output
    })
}

/// Get sha256sum hash from a byte stream
#[inline]
pub(crate) fn sha256sum(data: &[u8]) -> String {
    hex_encode(&openssl::sha::sha256(data))
}
