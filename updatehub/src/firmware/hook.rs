// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::firmware::{metadata_value::MetadataValue, Result};

use easy_process;
use slog_scope::error;
use std::{path::Path, str::FromStr};
use walkdir::WalkDir;

pub(crate) fn run_hook(path: &Path) -> Result<String> {
    if !path.exists() {
        return Ok("".into());
    }

    Ok(run_script(path.to_str().expect("Invalid path for hook"))?)
}

pub(crate) fn run_hooks_from_dir(path: &Path) -> Result<MetadataValue> {
    let mut outputs: Vec<String> = Vec::new();
    for entry in WalkDir::new(path).follow_links(true).min_depth(1).max_depth(1) {
        outputs.push(run_hook(entry?.path())?);
    }

    Ok(MetadataValue::from_str(&outputs.join("\n"))?)
}

pub(crate) fn run_script(cmd: &str) -> Result<String> {
    let output = easy_process::run(cmd)?;
    if !output.stderr.is_empty() {
        output.stderr.lines().for_each(|err| error!("{} (stderr): {}", cmd, err))
    }

    Ok(output.stdout.trim().into())
}
