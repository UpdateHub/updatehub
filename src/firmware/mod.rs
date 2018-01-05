//
// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: GPL-2.0
//

use walkdir::WalkDir;

use std::path::Path;
use std::str::FromStr;

use process;

mod metadata_value;
use self::metadata_value::MetadataValue;

const PRODUCT_UID_HOOK: &str = "product-uid";
const VERSION_HOOK: &str = "version";
const HARDWARE_HOOK: &str = "hardware";
const DEVICE_IDENTITY_DIR: &str = "device-identity.d";
const DEVICE_ATTRIBUTES_DIR: &str = "device-attributes.d";

/// Metadata stores the firmware metadata information. It is
/// organized in multiple fields.
///
/// The Metadata is created loading its information from the running
/// firmware. It uses the `load` method for that.
pub struct Metadata {
    /// Product UID which identifies the firmware on the management system
    pub product_uid: String,

    /// Version of firmware
    pub version: String,

    /// Hardware where the firmware is running
    pub hardware: String,

    /// Device Identity
    pub device_identity: MetadataValue,

    /// Device Attributes
    pub device_attributes: MetadataValue,
}

impl Metadata {
    pub fn new(path: &Path) -> Result<Metadata, Error> {
        let product_uid_hook = path.join(PRODUCT_UID_HOOK);
        let version_hook = path.join(VERSION_HOOK);
        let hardware_hook = path.join(HARDWARE_HOOK);
        let device_identity_dir = path.join(DEVICE_IDENTITY_DIR);
        let device_attributes_dir = path.join(DEVICE_ATTRIBUTES_DIR);

        let metadata = Metadata { product_uid: run_hook(&product_uid_hook)?,
                                  version: run_hook(&version_hook)?,
                                  hardware: run_hook(&hardware_hook)?,
                                  device_identity: run_hooks_from_dir(&device_identity_dir)?,
                                  device_attributes: run_hooks_from_dir(&device_attributes_dir)?, };

        if metadata.product_uid.is_empty() {
            return Err(Error::MissingProductUid);
        }

        if metadata.product_uid.len() != 64 {
            return Err(Error::InvalidProductUid);
        }

        if metadata.device_identity.is_empty() {
            return Err(Error::MissingDeviceIdentity);
        }

        Ok(metadata)
    }
}

#[derive(Debug)]
pub enum Error {
    Process(process::Error),
    InvalidProductUid,
    MissingProductUid,
    MissingDeviceIdentity,
}

impl From<process::Error> for Error {
    fn from(err: process::Error) -> Error {
        Error::Process(err)
    }
}

fn run_hook(path: &Path) -> Result<String, process::Error> {
    let mut buf: Vec<u8> = Vec::new();

    // check if path exists
    if !path.exists() {
        return Ok("".into());
    }

    let mut output = process::run(path.to_str().unwrap())?;

    buf.append(&mut output.stdout);
    if !output.stderr.is_empty() {
        let err = String::from_utf8_lossy(&output.stderr);
        for line in err.lines() {
            error!("{} (stderr): {}", path.display(), line);
        }
    }

    Ok(String::from_utf8_lossy(&buf[..]).trim().into())
}

fn run_hooks_from_dir(path: &Path) -> Result<MetadataValue, process::Error> {
    let mut outputs: Vec<String> = Vec::new();

    // check if path exists
    if !path.exists() {
        return Ok(MetadataValue::new());
    }

    for entry in WalkDir::new(path).follow_links(true)
                                   .min_depth(1)
                                   .max_depth(1)
    {
        let entry = entry?;
        let r = run_hook(entry.path())?;

        outputs.push(r);
    }

    Ok(MetadataValue::from_str(&outputs.join("\n"))?)
}

#[cfg(test)]
mod hooks {
    use super::*;
    use mktemp::Temp;

    fn create_hook(path: &Path, contents: &str, mode: u32) {
        use std::fs::File;
        use std::fs::create_dir_all;
        use std::fs::metadata;
        use std::io::Write;
        use std::os::unix::fs::PermissionsExt;

        // ensure path exists
        create_dir_all(path.parent().unwrap()).unwrap();

        let mut file = File::create(&path).unwrap();
        file.write(contents.as_bytes()).unwrap();

        let mut permissions = metadata(path).unwrap().permissions();
        permissions.set_mode(mode);
        file.set_permissions(permissions).unwrap();
    }

    #[test]
    fn run_multiple_hooks_in_a_dir() {
        let tmpdir = Temp::new_dir().unwrap();

        // create two scripts so we can test the parsing of output
        create_hook(&tmpdir.to_path_buf().join("hook1"),
                    "#!/bin/sh\necho key2=val2\necho key1=val1",
                    0o755);
        create_hook(&tmpdir.to_path_buf().join("hook2"),
                    "#!/bin/sh\necho key2=val4\necho key1=val3",
                    0o755);

        let fv = run_hooks_from_dir(&tmpdir.to_path_buf()).unwrap();

        assert_eq!(fv.keys().len(), 2);
        assert_eq!(fv.keys().collect::<Vec<_>>(), ["key1", "key2"]);
        assert_eq!(fv["key1"], ["val1", "val3"]);
        assert_eq!(fv["key2"], ["val2", "val4"]);
    }

    #[test]
    fn check_load_metadata() {
        use std::fs::remove_file;

        let tmpdir = Temp::new_dir().unwrap().to_path_buf();

        let product_uid_hook = tmpdir.join(PRODUCT_UID_HOOK);
        let version_hook = tmpdir.join(VERSION_HOOK);
        let hardware_hook = tmpdir.join(HARDWARE_HOOK);
        let device_identity_dir = tmpdir.join(DEVICE_IDENTITY_DIR).join("identity");
        let device_attributes_dir = tmpdir.join(DEVICE_ATTRIBUTES_DIR).join("attributes");

        let setup_metadata = || {
            // create fake hooks to be used to validate the load
            create_hook(&product_uid_hook,
                        "#!/bin/sh\necho 229ffd7e08721d716163fc81a2dbaf6c90d449f0a3b009b6a2defe8a0b0d7381",
                        0o755);
            create_hook(&version_hook, "#!/bin/sh\necho 1.1", 0o755);
            create_hook(&hardware_hook, "#!/bin/sh\necho board", 0o755);
            create_hook(&device_identity_dir,
                        "#!/bin/sh\necho id1=value1\necho id2=value2",
                        0o755);
            create_hook(&device_attributes_dir,
                        "#!/bin/sh\necho attr1=attrvalue1\necho attr2=attrvalue2",
                        0o755);
        };

        // check error with a invalid product uid
        setup_metadata();
        create_hook(&product_uid_hook, "#!/bin/sh\necho 123", 0o755);
        let metadata = Metadata::new(&tmpdir);
        assert!(metadata.is_err());

        // check error when lacks product uid
        setup_metadata();
        remove_file(&product_uid_hook).unwrap();
        let metadata = Metadata::new(&tmpdir);
        assert!(metadata.is_err());

        // check error when lacks device identity
        setup_metadata();
        remove_file(&device_identity_dir).unwrap();
        let metadata = Metadata::new(&tmpdir);
        assert!(metadata.is_err());

        // check if is still valid without device attributes
        setup_metadata();
        remove_file(&device_attributes_dir).unwrap();
        let metadata = Metadata::new(&tmpdir).unwrap();
        assert_eq!("229ffd7e08721d716163fc81a2dbaf6c90d449f0a3b009b6a2defe8a0b0d7381",
                   metadata.product_uid);
        assert_eq!("1.1", metadata.version);
        assert_eq!("board", metadata.hardware);
        assert_eq!(2, metadata.device_identity.len());
        assert_eq!(0, metadata.device_attributes.len());

        // complete metadata
        setup_metadata();
        let metadata = Metadata::new(&tmpdir).unwrap();
        assert_eq!("229ffd7e08721d716163fc81a2dbaf6c90d449f0a3b009b6a2defe8a0b0d7381",
                   metadata.product_uid);
        assert_eq!("1.1", metadata.version);
        assert_eq!("board", metadata.hardware);
        assert_eq!(2, metadata.device_identity.len());
        assert_eq!(2, metadata.device_attributes.len());
    }
}
