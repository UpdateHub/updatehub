// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
//

use std::path::Path;
use std::str::FromStr;

use failure::Error;
use walkdir::WalkDir;

use easy_process;
use firmware::metadata_value::MetadataValue;

pub fn run_hook(path: &Path) -> Result<String, Error> {
    if !path.exists() {
        return Ok("".into());
    }

    let output = easy_process::run(path.to_str().expect("Invalid path for hook"))?;
    if !output.stderr.is_empty() {
        output
            .stderr
            .lines()
            .for_each(|err| error!("{} (stderr): {}", path.display(), err))
    }

    Ok(output.stdout.trim().into())
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
