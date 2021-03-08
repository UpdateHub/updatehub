// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::firmware::hook::run_script;
use sdk::api::info::runtime_settings::InstallationSet;
use std::{fmt, str::FromStr};

const GET_SCRIPT: &str = "updatehub-active-get";
const SET_SCRIPT: &str = "updatehub-active-set";
const VALIDATE_SCRIPT: &str = "updatehub-active-validated";

#[derive(PartialEq, Debug, Copy, Clone)]
pub struct Set(pub InstallationSet);

impl FromStr for Set {
    type Err = super::Error;

    fn from_str(s: &str) -> super::Result<Self> {
        match s {
            "0" => Ok(Set(InstallationSet::A)),
            "1" => Ok(Set(InstallationSet::B)),
            _ => Err(super::Error::InvalidInstallSet),
        }
    }
}

impl fmt::Display for Set {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(
            f,
            "{}",
            match self {
                Set(InstallationSet::A) => "0",
                Set(InstallationSet::B) => "1",
            }
        )
    }
}

pub fn active() -> super::Result<Set> {
    run_script(GET_SCRIPT)?.parse()
}

pub fn inactive() -> super::Result<Set> {
    match active()? {
        Set(InstallationSet::A) => Ok(Set(InstallationSet::B)),
        Set(InstallationSet::B) => Ok(Set(InstallationSet::A)),
    }
}

pub fn swap_active() -> super::Result<()> {
    let _ = run_script(&format!("{} {}", SET_SCRIPT, inactive()?))?;
    Ok(())
}

pub fn validate() -> super::Result<()> {
    let _ = run_script(VALIDATE_SCRIPT)?;
    Ok(())
}

#[test]
fn as_str() {
    use pretty_assertions::assert_eq;
    assert_eq!("0", format!("{}", Set(InstallationSet::A)));
    assert_eq!("1", format!("{}", Set(InstallationSet::B)));
}

#[test]
fn works() {
    use crate::firmware::tests::create_fake_installation_set;
    use pretty_assertions::assert_eq;
    use std::env;
    use tempfile::tempdir;

    // create the fake backend
    let tmpdir = tempdir().unwrap();
    let tmpdir = tmpdir.path();
    env::set_var("PATH", format!("{}", &tmpdir.to_string_lossy()));

    // Create a fake backend using 0 as active. It must test the
    // following:
    // - active is A
    // - inactive is B
    // - swap works
    create_fake_installation_set(&tmpdir, 0);
    assert_eq!(active().unwrap(), Set(InstallationSet::A));
    assert_eq!(inactive().unwrap(), Set(InstallationSet::B));
    assert!(swap_active().is_ok());

    // Create a fake backend using 1 as active. It must test the
    // following:
    // - active is B
    // - inactive is A
    // - swap works
    create_fake_installation_set(&tmpdir, 1);
    assert_eq!(active().unwrap(), Set(InstallationSet::B));
    assert_eq!(inactive().unwrap(), Set(InstallationSet::A));
    assert!(swap_active().is_ok());
}
