// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::*;
use std::path::PathBuf;

#[cfg(test)]
use {pretty_assertions::assert_eq, tempfile::tempdir};

pub(crate) fn create_hook(path: PathBuf, contents: &str) {
    use std::{
        fs::{create_dir_all, metadata, File},
        io::Write,
        os::unix::fs::PermissionsExt,
        thread, time,
    };

    // ensure path exists
    create_dir_all(path.parent().unwrap()).unwrap();

    let mut file = File::create(&path).unwrap();
    file.write_all(contents.as_bytes()).unwrap();

    let mut permissions = metadata(path).unwrap().permissions();
    permissions.set_mode(0o755);
    file.set_permissions(permissions).unwrap();

    // This is needed because the filesystem may report it is complete
    // before it finishes writing it.
    thread::sleep(time::Duration::from_millis(50));
}

pub(crate) fn product_uid_hook(path: &Path) -> PathBuf {
    path.join(PRODUCT_UID_HOOK)
}

pub(crate) fn version_hook(path: &Path) -> PathBuf {
    path.join(VERSION_HOOK)
}

pub(crate) fn hardware_hook(path: &Path) -> PathBuf {
    path.join(HARDWARE_HOOK)
}

pub(crate) fn device_identity_dir(path: &Path) -> PathBuf {
    path.join(DEVICE_IDENTITY_DIR).join("identity")
}

pub(crate) fn device_attributes_dir(path: &Path) -> PathBuf {
    path.join(DEVICE_ATTRIBUTES_DIR).join("attributes")
}

#[cfg(test)]
pub(crate) fn create_fake_metadata() -> PathBuf {
    let tmpdir = tempdir().unwrap().path().to_path_buf();

    // create fake hooks to be used to validate the load
    create_hook(
        product_uid_hook(&tmpdir),
        "#!/bin/sh\necho 229ffd7e08721d716163fc81a2dbaf6c90d449f0a3b009b6a2defe8a0b0d7381",
    );
    create_hook(version_hook(&tmpdir), "#!/bin/sh\necho 1.1");
    create_hook(hardware_hook(&tmpdir), &"#!/bin/sh\necho board");
    create_hook(device_identity_dir(&tmpdir), &"#!/bin/sh\necho id1=value1\necho id2=value2");
    create_hook(
        device_attributes_dir(&tmpdir),
        "#!/bin/sh\necho attr1=attrvalue1\necho attr2=attrvalue2",
    );

    tmpdir
}

pub(crate) fn create_fake_installation_set(tmpdir: &Path, active: usize) {
    use std::{
        fs::{create_dir_all, metadata, File},
        io::Write,
        os::unix::fs::PermissionsExt,
    };

    const GET_SCRIPT: &str = "updatehub-active-get";
    const SET_SCRIPT: &str = "updatehub-active-set";
    const VALIDATE_SCRIPT: &str = "updatehub-active-validated";

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

    let mut file = File::create(&tmpdir.join(VALIDATE_SCRIPT)).unwrap();
    writeln!(file, "#!/bin/sh\nexit 0").unwrap();

    let mut permissions = metadata(tmpdir).unwrap().permissions();

    permissions.set_mode(0o755);
    file.set_permissions(permissions).unwrap();
}

pub(crate) fn create_fake_starup_callbacks(metadata_dir: &Path, output_file: &Path) {
    use std::{fs, io::Write, os::unix::fs::PermissionsExt};

    for script in &[VALIDATE_CALLBACK, ROLLBACK_CALLBACK] {
        let mut file = fs::OpenOptions::new()
            .write(true)
            .create(true)
            .open(&metadata_dir.join(script))
            .unwrap();
        writeln!(file, "#!/bin/sh\necho $0 >> {}", output_file.to_string_lossy()).unwrap();
        let mut permissions = fs::metadata(metadata_dir).unwrap().permissions();
        permissions.set_mode(0o755);
        file.set_permissions(permissions).unwrap();
    }
}

#[test]
fn run_multiple_hooks_in_a_dir() {
    let tmpdir = tempdir().unwrap().path().to_path_buf();

    // create two scripts so we can test the parsing of output
    create_hook(tmpdir.join("hook1"), "#!/bin/sh\necho key2=val2\necho key1=val1");
    create_hook(tmpdir.join("hook2"), "#!/bin/sh\necho key2=val4\necho key1=val3");

    let fv = run_hooks_from_dir(&tmpdir).unwrap();

    assert_eq!(fv.keys().len(), 2);
    assert_eq!(fv.keys().collect::<Vec<_>>(), ["key1", "key2"]);
    assert_eq!(fv["key1"], ["val1", "val3"]);
    assert_eq!(fv["key2"], ["val2", "val4"]);
}

#[test]
fn check_load_metadata() {
    use std::fs::remove_file;

    {
        let metadata_dir = create_fake_metadata();
        // check error with a invalid product uid
        create_hook(product_uid_hook(&metadata_dir), "#!/bin/sh\necho 123");
        let metadata = Metadata::from_path(&metadata_dir);
        assert!(metadata.is_err());
    }

    {
        // check error when lacks product uid
        let metadata_dir = create_fake_metadata();
        remove_file(product_uid_hook(&metadata_dir)).unwrap();
        let metadata = Metadata::from_path(&metadata_dir);
        assert!(metadata.is_err());
    }

    {
        // check error when lacks device identity
        let metadata_dir = create_fake_metadata();
        remove_file(device_identity_dir(&metadata_dir)).unwrap();
        let metadata = Metadata::from_path(&metadata_dir);
        assert!(metadata.is_err());
    }

    {
        // check if is still valid without device attributes
        let metadata_dir = create_fake_metadata();
        remove_file(device_attributes_dir(&metadata_dir)).unwrap();
        let metadata = Metadata::from_path(&metadata_dir).unwrap();
        assert_eq!(
            "229ffd7e08721d716163fc81a2dbaf6c90d449f0a3b009b6a2defe8a0b0d7381",
            metadata.product_uid
        );
        assert_eq!("1.1", metadata.version);
        assert_eq!("board", metadata.hardware);
        assert_eq!(2, metadata.device_identity.len());
        assert_eq!(0, metadata.device_attributes.len());
    }

    {
        // complete metadata
        let metadata_dir = create_fake_metadata();
        let metadata = Metadata::from_path(&metadata_dir).unwrap();
        assert_eq!(
            "229ffd7e08721d716163fc81a2dbaf6c90d449f0a3b009b6a2defe8a0b0d7381",
            metadata.product_uid
        );
        assert_eq!("1.1", metadata.version);
        assert_eq!("board", metadata.hardware);
        assert_eq!(2, metadata.device_identity.len());
        assert_eq!(2, metadata.device_attributes.len());
    }
}

#[cfg(test)]
const CALLBACK_STATE_NAME: &str = "test_state";

#[cfg(test)]
fn create_state_change_callback_hook(content: &str) -> tempfile::TempDir {
    let tmpdir = tempfile::tempdir().unwrap();
    create_hook(tmpdir.path().join(STATE_CHANGE_CALLBACK), content);
    tmpdir
}

#[test]
fn state_callback_cancel() {
    let script = "#!/bin/sh\necho cancel";
    let tmpdir = create_state_change_callback_hook(&script);
    assert_eq!(
        state_change_callback(&tmpdir.path(), CALLBACK_STATE_NAME).unwrap(),
        Transition::Cancel,
        "Unexpected result using content {:?}",
        script,
    );
}

#[test]
fn state_callback_continue_transition() {
    let script = "#!/bin/sh\necho ";
    let tmpdir = create_state_change_callback_hook(&script);
    assert_eq!(
        state_change_callback(&tmpdir.path(), CALLBACK_STATE_NAME).unwrap(),
        Transition::Continue,
        "Unexpected result using content {:?}",
        script,
    );
}

#[test]
fn state_callback_non_existing_hook() {
    assert_eq!(
        state_change_callback(&Path::new("/NaN"), CALLBACK_STATE_NAME).unwrap(),
        Transition::Continue,
        "Unexpected result for non-existing hook",
    );
}

#[test]
fn state_callback_is_error() {
    for script in &["#!/bin/sh\necho 123", "#!/bin/sh\necho 123\ncancel"] {
        let tmpdir = create_state_change_callback_hook(script);
        assert!(state_change_callback(&tmpdir.path(), CALLBACK_STATE_NAME).is_err());
    }
}
