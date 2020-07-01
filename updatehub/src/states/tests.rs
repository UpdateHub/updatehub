// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::*;
use crate::firmware::installation_set::Set;
use sdk::api::info::runtime_settings::InstallationSet;
use std::{fs, io};

#[test]
fn startup_without_upgrade() {
    let mut setup = crate::tests::TestEnvironment::build().finish();

    handle_startup_callbacks(&setup.settings.data, &mut setup.runtime_settings.data).unwrap();

    match fs::read_to_string(&setup.binaries.data) {
        Err(e) if e.kind() == io::ErrorKind::NotFound => (),
        Err(e) => panic!("Unexpected Error: {}", e),
        Ok(content) => panic!("Output file should be empty, instead we have: {}", content),
    }
}

#[test]
fn startup_with_normal_upgrade() {
    let mut setup = crate::tests::TestEnvironment::build().finish();
    let output_file_path = &setup.binaries.data;
    setup.runtime_settings.data.set_upgrading_to(Set(InstallationSet::A)).unwrap();

    handle_startup_callbacks(&setup.settings.data, &mut setup.runtime_settings.data).unwrap();

    assert!(
        fs::read_to_string(output_file_path).unwrap().contains("validate-callback"),
        "Validate callback was not called",
    );
    assert!(
        !fs::read_to_string(output_file_path).unwrap().contains("rollback-callback"),
        "Rollback callback should not be called",
    );
}

#[test]
fn startup_on_faulty_upgrade() {
    let mut setup = crate::tests::TestEnvironment::build().add_echo_binary("reboot").finish();
    let output_file_path = &setup.binaries.data;
    // Setup validation callback to always fail
    fs::write(
        setup.firmware.stored_path.join("validate-callback"),
        format!("#!/bin/sh\necho $0 >> {}\nexit 1", output_file_path.to_string_lossy()),
    )
    .unwrap();
    setup.runtime_settings.data.set_upgrading_to(Set(InstallationSet::A)).unwrap();

    handle_startup_callbacks(&setup.settings.data, &mut setup.runtime_settings.data).unwrap();

    assert!(
        fs::read_to_string(output_file_path).unwrap().contains("rollback-callback"),
        "Rollback callback was not called",
    );
    assert!(
        fs::read_to_string(output_file_path).unwrap().contains("validate-callback"),
        "Validate callback was not called",
    );
    assert!(
        fs::read_to_string(output_file_path).unwrap().contains("reboot"),
        "Reboot was not called",
    );
}

#[test]
fn startup_on_wrong_install_set() {
    let mut setup = crate::tests::TestEnvironment::build().finish();
    setup.runtime_settings.data.set_upgrading_to(Set(InstallationSet::B)).unwrap();

    handle_startup_callbacks(&setup.settings.data, &mut setup.runtime_settings.data).unwrap();

    match fs::read_to_string(&setup.binaries.data) {
        Err(e) if e.kind() == io::ErrorKind::NotFound => (),
        Err(e) => panic!("Unexpected Error: {}", e),
        Ok(content) => panic!("Output file should be empty, instead we have: {}", content),
    }
}
