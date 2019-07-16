// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

#[macro_use]
mod macros;
mod copy;
pub mod definitions;
mod flash;
mod imxkobs;
mod raw;
mod tarball;
#[cfg(test)]
mod test;
mod ubifs;

use crate::states::install::ObjectInstaller;

#[cfg(test)]
use self::test::Test;
use self::{copy::Copy, flash::Flash, imxkobs::Imxkobs, raw::Raw, tarball::Tarball, ubifs::Ubifs};

use crypto_hash::{Algorithm, Hasher};
use hex;
use serde::Deserialize;
use std::{
    fs::File,
    io::{BufReader, Read, Write},
    path::Path,
};

#[derive(Deserialize, PartialEq, Debug)]
#[serde(tag = "mode")]
#[serde(rename_all = "lowercase")]
pub(crate) enum Object {
    Copy(Box<Copy>),
    Flash(Box<Flash>),
    Imxkobs(Box<Imxkobs>),
    Raw(Box<Raw>),
    Tarball(Box<Tarball>),
    Ubifs(Box<Ubifs>),
    #[cfg(test)]
    Test(Box<Test>),
}

impl_object_for_object_types!(Copy, Flash, Imxkobs, Tarball, Ubifs, Raw);

impl ObjectInstaller for Object {
    fn check_requirements(&self) -> Result<(), failure::Error> {
        for_any_object!(self, o, { o.check_requirements() })
    }

    fn setup(&mut self) -> Result<(), failure::Error> {
        for_any_object!(self, o, { o.setup() })
    }

    fn install(&self, download_dir: std::path::PathBuf) -> Result<(), failure::Error> {
        for_any_object!(self, o, { o.install(download_dir) })
    }

    fn cleanup(&mut self) -> Result<(), failure::Error> {
        for_any_object!(self, o, { o.cleanup() })
    }
}

#[derive(PartialEq, Debug)]
pub(crate) enum ObjectStatus {
    Missing,
    Incomplete,
    Corrupted,
    Ready,
}

trait ObjectType {
    fn status(&self, download_dir: &Path) -> Result<ObjectStatus, failure::Error> {
        let object = download_dir.join(self.sha256sum());

        if !object.exists() {
            return Ok(ObjectStatus::Missing);
        }

        if object.metadata()?.len() < self.len() {
            return Ok(ObjectStatus::Incomplete);
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
            println!("{:?} {:?}", &self.sha256sum(), hex::encode(hasher.finish()));
            return Ok(ObjectStatus::Corrupted);
        }

        Ok(ObjectStatus::Ready)
    }

    fn filename(&self) -> &str;
    fn len(&self) -> u64;
    fn sha256sum(&self) -> &str;
}
