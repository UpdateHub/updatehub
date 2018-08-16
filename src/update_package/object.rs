// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
//

use Result;

use crypto_hash::{Algorithm, Hasher};
use hex;
use std::fs::File;
use std::io::BufReader;
use std::io::Read;
use std::io::Write;
use std::path::Path;

#[derive(Deserialize, PartialEq, Debug)]
#[serde(tag = "mode")]
#[serde(rename_all = "lowercase")]
pub enum Object {
    Test(Test),
}

#[derive(PartialEq, Debug)]
pub enum ObjectStatus {
    Missing,
    Incomplete,
    Corrupted,
    Ready,
}

trait ObjectType {
    fn status(&self, download_dir: &Path) -> Result<ObjectStatus> {
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

#[derive(Deserialize, PartialEq, Debug)]
#[serde(rename_all = "kebab-case")]
pub struct Test {
    filename: String,
    sha256sum: String,
    target: String,
    size: u64,
}

impl_object_for_object_types!(Test);
impl_object_type!(Test);
