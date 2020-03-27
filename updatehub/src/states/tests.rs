// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::*;
use crate::firmware::installation_set::Set;
use sdk::api::info::runtime_settings::InstallationSet;
use std::fs;
use tempfile::{NamedTempFile, TempDir};

fn fake_settings() -> (Settings, RuntimeSettings, NamedTempFile, TempDir) {
    use crate::{
        firmware::tests::{
            create_fake_installation_set, create_fake_starup_callbacks, create_hook,
        },
        update_package::tests::create_fake_settings,
    };
    use std::env;

    let output_file = NamedTempFile::new().unwrap();
    let firmware_dir = TempDir::new().unwrap();

    let mut settings = create_fake_settings();
    settings.firmware.metadata = firmware_dir.path().to_path_buf();
    let tmpdir = settings.update.download_dir.clone();
    create_fake_installation_set(&tmpdir, 0);
    create_hook(
        tmpdir.join("reboot"),
        &format!("#!/bin/sh\necho $0 >> {}", output_file.path().to_string_lossy()),
    );
    env::set_var("PATH", format!("{}", &tmpdir.to_string_lossy()));

    create_fake_starup_callbacks(&settings.firmware.metadata, output_file.path());

    let runtime_settings = RuntimeSettings::default();
    (settings, runtime_settings, output_file, firmware_dir)
}

#[test]
fn startup_without_upgrade() {
    let (settings, mut runtime_settings, output_file, ..) = fake_settings();

    handle_startup_callbacks(&settings, &mut runtime_settings).unwrap();
    assert!(
        fs::read_to_string(output_file.path()).unwrap().is_empty(),
        "Output file is empty as none of the callbacks should have been executed",
    );
}

#[test]
fn startup_with_normal_upgrade() {
    let (settings, mut runtime_settings, output_file, _guard_dir) = fake_settings();
    runtime_settings.set_upgrading_to(Set(InstallationSet::A)).unwrap();

    handle_startup_callbacks(&settings, &mut runtime_settings).unwrap();
    assert!(
        fs::read_to_string(output_file.path()).unwrap().contains("validate-callback"),
        "Validate callback was not called",
    );
    assert!(
        !fs::read_to_string(output_file.path()).unwrap().contains("rollback-callback"),
        "Rollback callback should not be called",
    );
}

#[test]
fn startup_on_faulty_upgrade() {
    let (settings, mut runtime_settings, output_file, _guard_dir) = fake_settings();
    // Setup validation callback to always fail
    fs::write(
        settings.firmware.metadata.join("validate-callback"),
        format!("#!/bin/sh\necho $0 >> {}\nexit 1", output_file.path().to_string_lossy()),
    )
    .unwrap();

    runtime_settings.set_upgrading_to(Set(InstallationSet::A)).unwrap();

    handle_startup_callbacks(&settings, &mut runtime_settings).unwrap();
    assert!(
        fs::read_to_string(output_file.path()).unwrap().contains("rollback-callback"),
        "Rollback callback was not called",
    );
    assert!(
        fs::read_to_string(output_file.path()).unwrap().contains("validate-callback"),
        "Validate callback was not called",
    );
    assert!(
        fs::read_to_string(output_file.path()).unwrap().contains("reboot"),
        "Reboot was not called",
    );
}

#[test]
fn startup_on_wrong_install_set() {
    let (settings, mut runtime_settings, output_file, _guard_dir) = fake_settings();
    runtime_settings.set_upgrading_to(Set(InstallationSet::B)).unwrap();

    handle_startup_callbacks(&settings, &mut runtime_settings).unwrap();
    assert!(
        !fs::read_to_string(output_file.path()).unwrap().contains("rollback-callback"),
        "Rollback callback should not be called",
    );
    assert!(
        !fs::read_to_string(output_file.path()).unwrap().contains("validate-callback"),
        "Validate callback should not be called",
    );
}
