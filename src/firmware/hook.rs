// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: GPL-2.0-only
// 

use std::io;
use std::path::Path;
use std::str::FromStr;

use checked_command;
use walkdir;
use walkdir::WalkDir;

use firmware::metadata_value::MetadataValue;
use process;

#[derive(Debug)]
pub enum Error {
    CheckedCommand(checked_command::Error),
    WalkDir(walkdir::Error),
    Io(io::Error),
}

impl From<checked_command::Error> for Error {
    fn from(err: checked_command::Error) -> Error {
        Error::CheckedCommand(err)
    }
}

impl From<walkdir::Error> for Error {
    fn from(err: walkdir::Error) -> Error {
        Error::WalkDir(err)
    }
}

impl From<io::Error> for Error {
    fn from(err: io::Error) -> Error {
        Error::Io(err)
    }
}

pub fn run_hook(path: &Path) -> Result<String, Error> {
    let mut buf: Vec<u8> = Vec::new();

    // check if path exists
    if !path.exists() {
        return Ok("".into());
    }

    let mut output = process::run(path.to_str().unwrap())?;

    buf.append(&mut output.stdout);
    if !output.stderr.is_empty() {
        let err = String::from_utf8_lossy(&output.stderr);
        for line in err.lines() {
            error!("{} (stderr): {}", path.display(), line);
        }
    }

    Ok(String::from_utf8_lossy(&buf[..]).trim().into())
}

pub fn run_hooks_from_dir(path: &Path) -> Result<MetadataValue, Error> {
    let mut outputs: Vec<String> = Vec::new();

    // check if path exists
    if !path.exists() {
        return Ok(MetadataValue::new());
    }

    for entry in WalkDir::new(path).follow_links(true)
                                   .min_depth(1)
                                   .max_depth(1)
    {
        let entry = entry?;
        let r = run_hook(entry.path())?;

        outputs.push(r);
    }

    Ok(MetadataValue::from_str(&outputs.join("\n"))?)
}
