// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::*;

use crate::{
    firmware::tests::{create_fake_metadata, FakeDevice},
    settings::Settings,
};

use mockito::{mock, Mock};
use serde_json::json;

pub(crate) enum FakeServer {
    NoUpdate,
    HasUpdate,
    ExtraPoll,
    ErrorOnce,
    InvalidHardware,
    ReportSuccess,
    ReportError,
}

pub(crate) fn create_mock_server(server: FakeServer) -> Mock {
    use crate::update_package::tests::{get_update_json, SHA256SUM};
    use mockito::Matcher;

    fn fake_device_reply_body(identity: usize, hardware: &str) -> Matcher {
        Matcher::Json(json!(
            {
                "product-uid": "229ffd7e08721d716163fc81a2dbaf6c90d449f0a3b009b6a2defe8a0b0d7381",
                "version": "1.1",
                "hardware": hardware,
                "device-identity": {
                    "id1":format!("value{}", identity),
                    "id2":"value2"
                },
                "device-attributes": {
                    "attr1":"attrvalue1",
                    "attr2":"attrvalue2"
                }
            }
        ))
    }

    match server {
        FakeServer::NoUpdate => mock("POST", "/upgrades")
            .match_header("Content-Type", "application/json")
            .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
            .match_body(fake_device_reply_body(1, "board"))
            .with_status(404)
            .create(),
        FakeServer::HasUpdate => mock("POST", "/upgrades")
            .match_header("Content-Type", "application/json")
            .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
            .match_body(fake_device_reply_body(2, "board"))
            .with_status(200)
            .with_body(&get_update_json(SHA256SUM).to_string())
            .create(),
        FakeServer::ExtraPoll => mock("POST", "/upgrades")
            .match_header("Content-Type", "application/json")
            .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
            .match_body(fake_device_reply_body(3, "board"))
            .with_status(200)
            .with_header("Add-Extra-Poll", "10")
            .create(),
        FakeServer::ErrorOnce => mock("POST", "/upgrades")
            .match_header("Content-Type", "application/json")
            .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
            .match_header("Api-Retries", "1")
            .match_body(fake_device_reply_body(1, "board"))
            .with_status(404)
            .create(),
        FakeServer::InvalidHardware => mock("POST", "/upgrades")
            .match_header("Content-Type", "application/json")
            .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
            .match_body(fake_device_reply_body(4, "invalid"))
            .with_status(200)
            .with_body(&get_update_json(SHA256SUM).to_string())
            .create(),
        FakeServer::ReportSuccess => mock("POST", "/report")
            .match_header("Content-Type", "application/json")
            .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
            .match_body(Matcher::Json(json!(
                {
                    "product-uid": "229ffd7e08721d716163fc81a2dbaf6c90d449f0a3b009b6a2defe8a0b0d7381",
                    "version": "1.1",
                    "hardware": "board",
                    "device-identity": {
                        "id1": "value2",
                        "id2": "value2"
                    },
                    "device-attributes": {
                        "attr1": "attrvalue1",
                        "attr2": "attrvalue2"
                    },
                    "status": "state",
                    "package-uid": "package-uid",
                }
            )))
            .with_status(200)
            .create(),
        FakeServer::ReportError => mock("POST", "/report")
            .match_header("Content-Type", "application/json")
            .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
            .match_body(Matcher::Json(json!(
                {
                    "product-uid": "229ffd7e08721d716163fc81a2dbaf6c90d449f0a3b009b6a2defe8a0b0d7381",
                    "version": "1.1",
                    "hardware": "board",
                    "device-identity": {
                        "id1": "value2",
                        "id2": "value2"
                    },
                    "device-attributes": {
                        "attr1": "attrvalue1",
                        "attr2": "attrvalue2"
                    },
                    "status": "state",
                    "package-uid": "package-uid",
                    "error-message": "errorMessage",
                    "previous-state": "previous-state"
                }
            )))
            .with_status(200)
            .create(),
    }
}

#[actix_rt::test]
async fn probe_requirements() {
    use crate::firmware::tests::{create_fake_metadata, FakeDevice};

    let mock = create_mock_server(FakeServer::NoUpdate);
    let _ = Api::new(&Settings::default().network.server_address)
        .probe(
            &RuntimeSettings::default(),
            &Metadata::from_path(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap(),
        )
        .await;
    mock.assert();
}

#[actix_rt::test]
async fn download_object() {
    let metadata = Metadata::from_path(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap();
    use pretty_assertions::assert_eq;
    use std::{fs::File, io::Read};
    use tempfile::tempdir;

    let m1 = mock(
        "GET",
        format!(
            "/products/{}/packages/{}/objects/{}",
            metadata.product_uid, "package_id", "object"
        )
        .as_str(),
    )
    .match_header("Content-Type", "application/json")
    .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
    .with_status(200)
    .with_body("1234")
    .create();

    let m2 = mock(
        "GET",
        format!(
            "/products/{}/packages/{}/objects/{}",
            metadata.product_uid, "package_id", "object"
        )
        .as_str(),
    )
    .match_header("Content-Type", "application/json")
    .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
    .match_header("Range", "bytes=3-")
    .with_status(200)
    .with_body("567890")
    .create();

    let mut settings = Settings::default();
    let tempdir = tempdir().unwrap();

    settings.update.download_dir = tempdir.path().to_path_buf();

    // Download the object.
    Api::new(&settings.network.server_address)
        .download_object(
            &metadata.product_uid,
            "package_id",
            &settings.update.download_dir,
            "object",
        )
        .await
        .expect("Failed to download the object.");

    // Verify it has been downloaded successfully.
    let mut downloaded = String::new();
    let _ = File::open(&settings.update.download_dir.join("object"))
        .expect("Failed to open destination object.")
        .read_to_string(&mut downloaded);

    m1.assert();

    assert_eq!(downloaded, "1234".to_string());

    // Download the remaining bytes of the object.
    Api::new(&settings.network.server_address)
        .download_object(
            &metadata.product_uid,
            "package_id",
            &settings.update.download_dir,
            "object",
        )
        .await
        .expect("Failed to download the object.");

    // Verify it has been downloaded successfully.
    let mut downloaded = String::new();
    let _ = File::open(&settings.update.download_dir.join("object"))
        .expect("Failed to open destination object.")
        .read_to_string(&mut downloaded);

    m2.assert();

    assert_eq!(downloaded, "1234567890".to_string());

    tempdir.close().expect("Fail to cleanup the tempdir");
}

#[actix_rt::test]
async fn report_success() {
    use crate::firmware::tests::{create_fake_metadata, FakeDevice};

    let mock = create_mock_server(FakeServer::ReportSuccess);
    let _ = Api::new(&Settings::default().network.server_address)
        .report(
            "state",
            &Metadata::from_path(&create_fake_metadata(FakeDevice::HasUpdate)).unwrap(),
            "package-uid",
            None,
            None,
            None,
        )
        .await;
    mock.assert();
}

#[actix_rt::test]
async fn report_error() {
    use crate::firmware::tests::{create_fake_metadata, FakeDevice};

    let mock = create_mock_server(FakeServer::ReportError);
    let _ = Api::new(&Settings::default().network.server_address)
        .report(
            "state",
            &Metadata::from_path(&create_fake_metadata(FakeDevice::HasUpdate)).unwrap(),
            "package-uid",
            Some("previous-state"),
            Some("errorMessage".into()),
            None,
        )
        .await;
    mock.assert();
}
