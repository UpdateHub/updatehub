// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::firmware::hook::run_script;

use failure::bail;
use std::{fmt, result, str::FromStr};

const GET_SCRIPT: &str = "updatehub-active-get";
const SET_SCRIPT: &str = "updatehub-active-set";

#[derive(PartialEq, Debug, Copy, Clone)]
pub enum Set {
    A,
    B,
}

impl FromStr for Set {
    type Err = failure::Error;

    fn from_str(s: &str) -> result::Result<Self, Self::Err> {
        match s.parse::<u8>() {
            Ok(0) => Ok(Set::A),
            Ok(1) => Ok(Set::B),
            Ok(v) => bail!("{} is a invalid value. The only know ones are 0 or 1", v),
            Err(e) => Err(e.into()),
        }
    }
}

impl fmt::Display for Set {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(
            f,
            "{}",
            match self {
                Set::A => "0",
                Set::B => "1",
            }
        )
    }
}

pub fn active() -> Result<Set, failure::Error> {
    Ok(run_script(GET_SCRIPT)?.parse()?)
}

pub fn inactive() -> Result<Set, failure::Error> {
    match active()? {
        Set::A => Ok(Set::B),
        Set::B => Ok(Set::A),
    }
}

pub fn swap_active() -> Result<(), failure::Error> {
    let _ = run_script(&format!("{} {}", SET_SCRIPT, inactive()?))?;
    Ok(())
}

#[test]
fn as_str() {
    assert_eq!("0", format!("{}", Set::A));
    assert_eq!("1", format!("{}", Set::B));
}

#[test]
fn works() {
    use std::env;
    use tempfile::tempdir;

    // create the fake backend
    let tmpdir = tempdir().unwrap();
    let tmpdir = tmpdir.path();
    env::set_var("PATH", format!("{}", &tmpdir.to_string_lossy()));

    let create_fake_backend = |active: usize| {
        use std::{
            fs::{create_dir_all, metadata, File},
            io::Write,
            os::unix::fs::PermissionsExt,
        };

        create_dir_all(&tmpdir).unwrap();

        let mut file = File::create(&tmpdir.join(GET_SCRIPT)).unwrap();
        writeln!(file, "#!/bin/sh\necho {}", active).unwrap();

        let mut permissions = metadata(tmpdir).unwrap().permissions();

        permissions.set_mode(0o755);
        file.set_permissions(permissions).unwrap();

        let mut file = File::create(&tmpdir.join(SET_SCRIPT)).unwrap();
        writeln!(file, "#!/bin/sh\nexit 0").unwrap();

        let mut permissions = metadata(tmpdir).unwrap().permissions();
        permissions.set_mode(0o755);
        file.set_permissions(permissions).unwrap();
    };

    // Create a fake backend using 0 as active. It must test the
    // following:
    // - active is A
    // - inactive is B
    // - swap works
    create_fake_backend(0);
    assert_eq!(active().unwrap(), Set::A);
    assert_eq!(inactive().unwrap(), Set::B);
    assert!(swap_active().is_ok());

    // Create a fake backend using 1 as active. It must test the
    // following:
    // - active is B
    // - inactive is A
    // - swap works
    create_fake_backend(1);
    assert_eq!(active().unwrap(), Set::B);
    assert_eq!(inactive().unwrap(), Set::A);
    assert!(swap_active().is_ok());
}
