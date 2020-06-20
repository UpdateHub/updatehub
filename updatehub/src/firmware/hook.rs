// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::firmware::Result;
use sdk::api::info::firmware::MetadataValue;
use slog_scope::error;
use std::{io, path::Path};
use walkdir::WalkDir;

pub(crate) fn run_hook(path: &Path) -> Result<String> {
    if !path.exists() {
        return Ok("".into());
    }

    Ok(run_script(path.to_str().expect("invalid path for hook"))?)
}

pub(crate) fn run_hooks_from_dir(path: &Path) -> Result<MetadataValue> {
    let mut outputs: Vec<String> = Vec::new();
    for entry in WalkDir::new(path).follow_links(true).min_depth(1).max_depth(1) {
        outputs.push(run_hook(entry?.path())?);
    }

    Ok(metadata_value_from_str(&outputs.join("\n"))?)
}

pub(crate) fn run_script(cmd: &str) -> Result<String> {
    let output = easy_process::run(cmd)?;
    if !output.stderr.is_empty() {
        output.stderr.lines().for_each(|err| error!("{} (stderr): {}", cmd, err))
    }

    Ok(output.stdout.trim().into())
}

fn metadata_value_from_str(s: &str) -> io::Result<MetadataValue> {
    let mut values = Vec::new();
    for line in s.lines() {
        let v: Vec<_> = line.splitn(2, '=').map(|v| v.trim().to_string()).collect();
        if v.len() != 2 {
            return Err(io::Error::new(
                io::ErrorKind::InvalidInput,
                format!("invalid format for value '{:?}', the <key>=<value> output is expected", v),
            ));
        }

        values.push((v[0].clone(), v[1].clone()));
    }
    values.sort();

    let mut mv = MetadataValue::default();
    for (k, v) in values {
        mv.0.entry(k).and_modify(|e| e.push(v.clone())).or_insert_with(|| vec![v]);
    }

    Ok(mv)
}

#[test]
fn valid() {
    use pretty_assertions::assert_eq;
    let v = metadata_value_from_str("key1=v1\nkey=b\nnv=\nkey=a").unwrap();

    assert_eq!(v.keys().len(), 3);
    assert_eq!(v.keys().collect::<Vec<_>>(), ["key", "key1", "nv"]);
    assert_eq!(v["key1"], ["v1"]);
    assert_eq!(v["key"], ["a", "b"]);
    assert_eq!(v["nv"], [""]);
}

#[test]
fn invalid() {
    assert!(metadata_value_from_str("\n").is_err());
    assert!(metadata_value_from_str("key").is_err());
}
