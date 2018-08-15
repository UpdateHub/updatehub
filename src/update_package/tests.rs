// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
//

use super::*;
use serde_json;

const SHA256SUM: &str = "c775e7b757ede630cd0aa1113bd102661ab38829ca52a6422ab782862f268646";

pub fn get_update_json() -> serde_json::Value {
    json!(
        {
            "product-uid": "0123456789",
            "version": "1.0",
            "supported-hardware": ["board"],
            "objects":
            [
                {
                    "mode": "test",
                    "filename": "testfile",
                    "target": "/dev/device1",
                    "sha256sum": SHA256SUM,
                    "size": 10
                }
            ]
        }
    )
}

pub fn get_update_package() -> UpdatePackage {
    serde_json::from_value(get_update_json()).unwrap()
}

pub fn create_fake_object(settings: &Settings) {
    use std::fs::create_dir_all;
    use std::fs::File;
    use std::io::Write;

    let dir = &settings.update.download_dir;

    // ensure path exists
    create_dir_all(&dir).unwrap();

    let _ = File::create(&dir.join(SHA256SUM))
        .unwrap()
        .write_all(b"1234567890")
        .unwrap();
}

pub fn create_fake_settings() -> Settings {
    use tempfile::tempdir;

    let mut settings = Settings::default();
    settings.update.download_dir = tempdir().unwrap().path().to_path_buf();
    settings
}

#[test]
fn missing_object_file() {
    let u = get_update_package();
    let settings = create_fake_settings();

    assert_eq!(u.filter_objects(&settings, &ObjectStatus::Missing).len(), 1);
}

#[test]
fn complete_object_file() {
    let u = get_update_package();
    let settings = create_fake_settings();

    create_fake_object(&settings);

    assert!(
        u.filter_objects(&settings, &ObjectStatus::Missing)
            .is_empty()
    );

    assert!(
        u.filter_objects(&settings, &ObjectStatus::Incomplete)
            .is_empty()
    );

    assert!(
        u.filter_objects(&settings, &ObjectStatus::Corrupted)
            .is_empty()
    );

    assert_eq!(
        u.filter_objects(&settings, &ObjectStatus::Ready)
            .iter()
            .count(),
        1
    );
}
