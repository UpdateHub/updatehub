// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::*;

use pretty_assertions::assert_eq;
use serde_json::json;

pub(crate) const SHA256SUM: &str =
    "c775e7b757ede630cd0aa1113bd102661ab38829ca52a6422ab782862f268646";
pub(crate) const OBJECT: &[u8] = b"1234567890";

pub(crate) fn get_update_json(sha256sum: &str) -> serde_json::Value {
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
                        "sha256sum": sha256sum,
                        "size": 10
                    }
                ],
                [
                    {
                        "mode": "test",
                        "filename": "testfile",
                        "target": "/dev/device2",
                        "sha256sum": sha256sum,
                        "size": 10
                    }
                ]
            ]
        }
    )
}

pub(crate) fn get_update_package() -> UpdatePackage {
    serde_json::from_value(get_update_json(SHA256SUM))
        .map_err(|e| println!("{:?}", e))
        .unwrap()
}

pub(crate) fn get_update_package_with_shasum(shasum: &str) -> UpdatePackage {
    serde_json::from_value(get_update_json(shasum)).unwrap()
}

pub(crate) fn create_fake_object(body: &[u8], shasum: &str, settings: &Settings) {
    use std::{
        fs::{create_dir_all, File},
        io::Write,
    };

    let dir = &settings.update.download_dir;

    // ensure path exists
    create_dir_all(&dir).unwrap();

    File::create(&dir.join(shasum))
        .unwrap()
        .write_all(body)
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
            .filter_objects(&settings, InstallationSet::A, object::info::Status::Missing)
            .len(),
        1
    );
}

#[test]
fn complete_object_file() {
    let update_package = get_update_package();
    let settings = create_fake_settings();

    create_fake_object(OBJECT, SHA256SUM, &settings);

    assert!(update_package
        .filter_objects(&settings, InstallationSet::A, object::info::Status::Missing)
        .is_empty());

    assert!(update_package
        .filter_objects(
            &settings,
            InstallationSet::A,
            object::info::Status::Incomplete
        )
        .is_empty());

    assert!(update_package
        .filter_objects(
            &settings,
            InstallationSet::A,
            object::info::Status::Corrupted
        )
        .is_empty());

    assert_eq!(
        update_package
            .filter_objects(&settings, InstallationSet::A, object::info::Status::Ready)
            .iter()
            .count(),
        1
    );
}
