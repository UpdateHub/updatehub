// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::Result;
use crypto_hash::{Algorithm, Hasher};
use hex;
use pkg_schema::{objects, Object};
use std::{
    fs::File,
    io::{BufReader, Read, Write},
    path::Path,
};

#[derive(PartialEq, Debug)]
pub(crate) enum Status {
    Missing,
    Incomplete,
    Corrupted,
    Ready,
}

impl_object_info!(objects::Copy);
impl_object_info!(objects::Flash);
impl_object_info!(objects::Imxkobs);
impl_object_info!(objects::Tarball);
impl_object_info!(objects::Ubifs);
impl_object_info!(objects::Raw);
impl_object_info!(objects::Test);

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
        let mut hasher = Hasher::new(Algorithm::SHA256);
        loop {
            let len = reader.read(&mut buf)?;
            hasher.write_all(&buf[..len])?;

            if len == 0 {
                break;
            }
        }

        if hex::encode(hasher.finish()) != self.sha256sum() {
            return Ok(Status::Corrupted);
        }

        Ok(Status::Ready)
    }

    fn filename(&self) -> &str;
    fn len(&self) -> u64;
    fn sha256sum(&self) -> &str;
}
