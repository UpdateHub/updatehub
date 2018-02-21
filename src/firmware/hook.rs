// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
// 

use std::io;
use std::path::Path;
use std::str::FromStr;

use checked_command;
use failure::Error;
use walkdir;
use walkdir::WalkDir;

use firmware::metadata_value::MetadataValue;
use process;

#[derive(Fail, Debug)]
pub enum HookError {
    #[fail(display = "Failed executing the command {}", _0)]
    CheckedCommand(#[cause] checked_command::Error),
    #[fail(display = "Failed to process the directory {}", _0)]
    WalkDir(#[cause] walkdir::Error),
    #[fail(display = "Failed to write/read {}", _0)]
    Io(#[cause] io::Error),
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
        return Ok(MetadataValue::default());
    }

    for entry in WalkDir::new(path)
        .follow_links(true)
        .min_depth(1)
        .max_depth(1)
    {
        let entry = entry?;
        let r = run_hook(entry.path())?;

        outputs.push(r);
    }

    Ok(MetadataValue::from_str(&outputs.join("\n"))?)
}
