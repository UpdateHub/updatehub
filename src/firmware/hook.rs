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
    if !path.exists() {
        return Ok("".into());
    }

    let output = process::run(path.to_str().unwrap())?;
    if !output.stderr.is_empty() {
        String::from_utf8_lossy(&output.stderr)
            .lines()
            .for_each(|err| error!("{} (stderr): {}", path.display(), err))
    }
    Ok(String::from_utf8_lossy(&output.stdout[..]).trim().into())
}

pub fn run_hooks_from_dir(path: &Path) -> Result<MetadataValue, Error> {
    let mut outputs: Vec<String> = Vec::new();
    for entry in WalkDir::new(path)
        .follow_links(true)
        .min_depth(1)
        .max_depth(1)
    {
        outputs.push(run_hook(entry?.path())?);
    }

    Ok(MetadataValue::from_str(&outputs.join("\n"))?)
}
