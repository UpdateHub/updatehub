// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
//

use Result;

use firmware::metadata_value::MetadataValue;
use hook;
use std::path::Path;
use std::str::FromStr;
use walkdir::WalkDir;

pub fn run_hooks_from_dir(path: &Path) -> Result<MetadataValue> {
    let mut outputs: Vec<String> = Vec::new();
    for entry in WalkDir::new(path)
        .follow_links(true)
        .min_depth(1)
        .max_depth(1)
    {
        outputs.push(hook::run_hook(entry?.path())?);
    }

    Ok(MetadataValue::from_str(&outputs.join("\n"))?)
}
