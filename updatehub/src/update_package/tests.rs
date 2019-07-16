// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::*;

use pretty_assertions::assert_eq;
use serde_json::json;

const SHA256SUM: &str = "c775e7b757ede630cd0aa1113bd102661ab38829ca52a6422ab782862f268646";

pub(crate) fn get_update_json() -> serde_json::Value {
    json!(
        {
            "product-uid": "0123456789",
            "version": "1.0",
            "supported-hardware": ["board"],
            "objects":
            [
                [
                    {
                        "mode": "test",
                        "filename": "testfile",
                        "target": "/dev/device1",
                        "sha256sum": SHA256SUM,
                        "size": 10
                    }
                ],
                [
                    {
                        "mode": "test",
                        "filename": "testfile",
                        "target": "/dev/device2",
                        "sha256sum": SHA256SUM,
                        "size": 10
                    }
                ]
            ]
        }
    )
}

pub(crate) fn get_update_package() -> UpdatePackage {
    serde_json::from_value(get_update_json())
        .map_err(|e| println!("{:?}", e))
        .unwrap()
}

pub(crate) fn create_fake_object(settings: &Settings) {
    use std::{
        fs::{create_dir_all, File},
        io::Write,
    };

    let dir = &settings.update.download_dir;

    // ensure path exists
    create_dir_all(&dir).unwrap();

    File::create(&dir.join(SHA256SUM))
        .unwrap()
        .write_all(b"1234567890")
        .unwrap();
}

pub(crate) fn create_fake_settings() -> Settings {
    use tempfile::tempdir;

    let mut settings = Settings::default();
    settings.update.download_dir = tempdir().unwrap().path().to_path_buf();
    settings
}

#[test]
fn missing_object_file() {
    let update_package = get_update_package();
    let settings = create_fake_settings();

    assert_eq!(
        update_package
            .filter_objects(&settings, InstallationSet::A, &ObjectStatus::Missing)
            .len(),
        1
    );
}

#[test]
fn complete_object_file() {
    let update_package = get_update_package();
    let settings = create_fake_settings();

    create_fake_object(&settings);

    assert!(update_package
        .filter_objects(&settings, InstallationSet::A, &ObjectStatus::Missing)
        .is_empty());

    assert!(update_package
        .filter_objects(&settings, InstallationSet::A, &ObjectStatus::Incomplete)
        .is_empty());

    assert!(update_package
        .filter_objects(&settings, InstallationSet::A, &ObjectStatus::Corrupted)
        .is_empty());

    assert_eq!(
        update_package
            .filter_objects(&settings, InstallationSet::A, &ObjectStatus::Ready)
            .iter()
            .count(),
        1
    );
}
