// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
//

use Result;

use easy_process;
use std::path::Path;

pub(crate) fn run_hook(path: &Path) -> Result<String> {
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
