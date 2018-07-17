// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
//

use super::*;
use std::path::PathBuf;
use tempfile::tempdir;

pub fn create_hook(path: PathBuf, contents: &str) {
    use std::fs::create_dir_all;
    use std::fs::metadata;
    use std::fs::File;
    use std::io::Write;
    use std::os::unix::fs::PermissionsExt;
    use std::{thread, time};

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

pub fn product_uid_hook(path: &Path) -> PathBuf {
    path.join(PRODUCT_UID_HOOK)
}

pub fn version_hook(path: &Path) -> PathBuf {
    path.join(VERSION_HOOK)
}

pub fn hardware_hook(path: &Path) -> PathBuf {
    path.join(HARDWARE_HOOK)
}

pub fn device_identity_dir(path: &Path) -> PathBuf {
    path.join(DEVICE_IDENTITY_DIR).join("identity")
}

pub fn device_attributes_dir(path: &Path) -> PathBuf {
    path.join(DEVICE_ATTRIBUTES_DIR).join("attributes")
}

pub enum FakeDevice {
    NoUpdate,
    HasUpdate,
    ExtraPoll,
    InvalidHardware,
}

pub fn create_fake_metadata(device: FakeDevice) -> PathBuf {
    let tmpdir = tempdir().unwrap().path().to_path_buf();

    // create fake hooks to be used to validate the load
    create_hook(
        product_uid_hook(&tmpdir),
        "#!/bin/sh\necho 229ffd7e08721d716163fc81a2dbaf6c90d449f0a3b009b6a2defe8a0b0d7381",
    );
    create_hook(version_hook(&tmpdir), "#!/bin/sh\necho 1.1");
    create_hook(
        hardware_hook(&tmpdir),
        &format!(
            "#!/bin/sh\necho {}",
            match device {
                FakeDevice::InvalidHardware => "invalid",
                _ => "board",
            }
        ),
    );
    create_hook(
        device_identity_dir(&tmpdir),
        &format!(
            "#!/bin/sh\necho id1=value{}\necho id2=value2",
            match device {
                FakeDevice::NoUpdate => 1,
                FakeDevice::HasUpdate => 2,
                FakeDevice::ExtraPoll => 3,
                FakeDevice::InvalidHardware => 4,
            }
        ),
    );
    create_hook(
        device_attributes_dir(&tmpdir),
        "#!/bin/sh\necho attr1=attrvalue1\necho attr2=attrvalue2",
    );

    tmpdir
}

#[test]
fn run_multiple_hooks_in_a_dir() {
    let tmpdir = tempdir().unwrap().path().to_path_buf();

    // create two scripts so we can test the parsing of output
    create_hook(
        tmpdir.join("hook1"),
        "#!/bin/sh\necho key2=val2\necho key1=val1",
    );
    create_hook(
        tmpdir.join("hook2"),
        "#!/bin/sh\necho key2=val4\necho key1=val3",
    );

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
        let metadata_dir = create_fake_metadata(FakeDevice::NoUpdate);
        // check error with a invalid product uid
        create_hook(product_uid_hook(&metadata_dir), "#!/bin/sh\necho 123");
        let metadata = Metadata::new(&metadata_dir);
        assert!(metadata.is_err());
    }

    {
        // check error when lacks product uid
        let metadata_dir = create_fake_metadata(FakeDevice::NoUpdate);
        remove_file(product_uid_hook(&metadata_dir)).unwrap();
        let metadata = Metadata::new(&metadata_dir);
        assert!(metadata.is_err());
    }

    {
        // check error when lacks device identity
        let metadata_dir = create_fake_metadata(FakeDevice::NoUpdate);
        remove_file(device_identity_dir(&metadata_dir)).unwrap();
        let metadata = Metadata::new(&metadata_dir);
        assert!(metadata.is_err());
    }

    {
        // check if is still valid without device attributes
        let metadata_dir = create_fake_metadata(FakeDevice::NoUpdate);
        remove_file(device_attributes_dir(&metadata_dir)).unwrap();
        let metadata = Metadata::new(&metadata_dir).unwrap();
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
        let metadata_dir = create_fake_metadata(FakeDevice::NoUpdate);
        let metadata = Metadata::new(&metadata_dir).unwrap();
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
