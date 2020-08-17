// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::Result;
use crate::utils;
use openssl::sha::Sha256;
use pkg_schema::{
    objects::{Copy, Flash, Imxkobs, Raw, Tarball, Test, Ubifs},
    Object,
};
use std::{
    fs::File,
    io::{BufReader, Read},
    path::Path,
};

#[derive(PartialEq, Debug)]
pub(crate) enum Status {
    Missing,
    Incomplete,
    Corrupted,
    Ready,
}

impl_compressed_object_info!(Copy);
impl_compressed_object_info!(Raw);
impl_compressed_object_info!(Ubifs);
impl_object_info!(Flash);
impl_object_info!(Imxkobs);
impl_object_info!(Tarball);
impl_object_info!(Test);

impl_object_for_object_types!(Copy, Flash, Imxkobs, Tarball, Ubifs, Raw, Test);

pub(crate) trait Info {
    fn status(&self, download_dir: &Path) -> Result<Status> {
        let object = download_dir.join(self.sha256sum());

        if !object.exists() {
            return Ok(Status::Missing);
        }

        if object.metadata()?.len() < self.len() {
            return Ok(Status::Incomplete);
        }

        let mut buf = [0; 1024];
        let mut reader = BufReader::new(File::open(object)?);
        let mut hasher = Sha256::new();
        loop {
            let len = reader.read(&mut buf)?;
            hasher.update(&buf[..len]);

            if len == 0 {
                break;
            }
        }

        if utils::hex_encode(&hasher.finish()) != self.sha256sum() {
            return Ok(Status::Corrupted);
        }

        Ok(Status::Ready)
    }

    fn mode(&self) -> String;
    fn filename(&self) -> &str;
    fn len(&self) -> u64;
    fn sha256sum(&self) -> &str;
    fn required_install_size(&self) -> u64;
}
