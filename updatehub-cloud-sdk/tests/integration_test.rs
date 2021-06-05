// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use updatehub_cloud_sdk as sdk;

use mockito::{mock, Mock};
use serde_json::json;
use std::collections::BTreeMap;

enum FakeServer {
    NoUpdate,
    HasUpdate,
    ExtraPoll,
    WithRetry,
    ReportSuccess,
    ReportError,
    DownloadInParts,
}

fn create_mock_server(server: FakeServer) -> (String, Vec<Mock>) {
    use mockito::Matcher;

    let json_update = json!({
        "product": "0123456789",
        "version": "1.0",
        "supported-hardware": ["board"],
        "objects":
        [
            [
                {
                    "mode": "test",
                    "filename": "testfile",
                    "target": "/dev/device1",
                    "sha256sum": "c775e7b757ede630cd0aa1113bd102661ab38829ca52a6422ab782862f268646",
                    "size": 10,
                    "force-check-requirements-fail": false
                }
            ],
            [
                {
                    "mode": "test",
                    "filename": "testfile",
                    "target": "/dev/device2",
                    "sha256sum": "c775e7b757ede630cd0aa1113bd102661ab38829ca52a6422ab782862f268646",
                    "size": 10,
                    "force-check-requirements-fail": false
                }
            ]
        ]
    });

    let reply_body = Matcher::Json(json!({
        "product-uid": "229ffd7e08721d716163fc81a2dbaf6c90d449f0a3b009b6a2defe8a0b0d7381",
        "version": "1.1",
        "hardware": "board",
        "device-identity": {
            "id1":"value1",
            "id2":"value2"
        },
        "device-attributes": {
            "attr1":"attrvalue1",
            "attr2":"attrvalue2"
        }
    }));

    let mocks = match server {
        FakeServer::NoUpdate => vec![mock("POST", "/upgrades")
            .match_header("Content-Type", "application/json")
            .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
            .match_body(reply_body)
            .with_status(404)
            .create()],
        FakeServer::HasUpdate => vec![mock("POST", "/upgrades")
            .match_header("Content-Type", "application/json")
            .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
            .match_body(reply_body)
            .with_status(200)
            .with_header("UH-Signature", &openssl::base64::encode_block(b"some_signature"))
            .with_body(&json_update.to_string())
            .create()],
        FakeServer::ExtraPoll => vec![mock("POST", "/upgrades")
            .match_header("Content-Type", "application/json")
            .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
            .match_body(reply_body)
            .with_status(200)
            .with_header("Add-Extra-Poll", "10")
            .create()],
        FakeServer::WithRetry => vec![mock("POST", "/upgrades")
            .match_header("Content-Type", "application/json")
            .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
            .match_header("Api-Retries", "1")
            .match_body(reply_body)
            .with_status(404)
            .create()],
        FakeServer::ReportSuccess => vec![mock("POST", "/report")
            .match_header("Content-Type", "application/json")
            .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
            .match_body(Matcher::Json(json!(
                {
                    "product-uid": "229ffd7e08721d716163fc81a2dbaf6c90d449f0a3b009b6a2defe8a0b0d7381",
                    "version": "1.1",
                    "hardware": "board",
                    "device-identity": {
                        "id1": "value1",
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
            .create()],
        FakeServer::ReportError => vec![mock("POST", "/report")
            .match_header("Content-Type", "application/json")
            .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
            .match_body(Matcher::Json(json!(
                {
                    "product-uid": "229ffd7e08721d716163fc81a2dbaf6c90d449f0a3b009b6a2defe8a0b0d7381",
                    "version": "1.1",
                    "hardware": "board",
                    "device-identity": {
                        "id1": "value1",
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
            .create()],
        FakeServer::DownloadInParts => vec![
            mock(
                "GET",
                format!(
                    "/products/{}/packages/{}/objects/{}",
                    FakeMetadata::PRODUCT_UID, "package_id", "object"
                )
                    .as_str(),
            )
                .match_header("Content-Type", "application/json")
                .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
                .with_status(200)
                .with_body("1234")
                .create(),
            mock(
                "GET",
                format!(
                    "/products/{}/packages/{}/objects/{}",
                    FakeMetadata::PRODUCT_UID, "package_id", "object"
                )
                    .as_str(),
            )
                .match_header("Content-Type", "application/json")
                .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
                .match_header("Range", "bytes=3-")
                .with_status(200)
                .with_body("567890")
                .create()
        ],
    };

    (mockito::server_url(), mocks)
}

struct FakeMetadata {
    identity: BTreeMap<String, Vec<String>>,
    attributes: BTreeMap<String, Vec<String>>,
}

impl FakeMetadata {
    const PRODUCT_UID: &'static str =
        "229ffd7e08721d716163fc81a2dbaf6c90d449f0a3b009b6a2defe8a0b0d7381";

    fn new() -> Self {
        let mut identity = BTreeMap::default();
        let mut attributes = BTreeMap::default();
        identity.insert(String::from("id1"), vec![String::from("value1")]);
        identity.insert(String::from("id2"), vec![String::from("value2")]);
        attributes.insert(String::from("attr1"), vec![String::from("attrvalue1")]);
        attributes.insert(String::from("attr2"), vec![String::from("attrvalue2")]);
        FakeMetadata { identity, attributes }
    }

    fn get(&self) -> sdk::api::FirmwareMetadata<'_> {
        sdk::api::FirmwareMetadata {
            product_uid: Self::PRODUCT_UID,
            version: "1.1",
            hardware: "board",
            device_identity: sdk::api::MetadataValue(&self.identity),
            device_attributes: sdk::api::MetadataValue(&self.attributes),
        }
    }
}

#[async_std::test]
async fn direct_get_invalid_url() {
    let res = sdk::get("http://foo.bar:---", &mut async_std::io::sink()).await;
    assert!(res.is_err());
}

#[async_std::test]
async fn probe_requirements() {
    let (url, mocks) = create_mock_server(FakeServer::NoUpdate);
    sdk::Client::new(&url).probe(0, FakeMetadata::new().get()).await.unwrap();
    mocks.iter().for_each(Mock::assert);
}

#[async_std::test]
async fn probe_invalid_url() {
    let res = sdk::Client::new("http://foo.bar:---").probe(0, FakeMetadata::new().get()).await;
    assert!(res.is_err());
}

#[async_std::test]
async fn probe_with_retry() {
    let (url, mocks) = create_mock_server(FakeServer::WithRetry);
    sdk::Client::new(&url).probe(1, FakeMetadata::new().get()).await.unwrap();
    mocks.iter().for_each(Mock::assert);
}

#[async_std::test]
async fn probe_response_with_signature() {
    use sdk::api::ProbeResponse;
    let (url, mocks) = create_mock_server(FakeServer::HasUpdate);
    let response = sdk::Client::new(&url).probe(0, FakeMetadata::new().get()).await.unwrap();
    match response {
        ProbeResponse::Update(_, Some(signature)) => assert_eq!(
            signature,
            sdk::api::Signature::from_base64_str(&openssl::base64::encode_block(b"some_signature"))
                .unwrap()
        ),
        ProbeResponse::Update(_, None) => panic!("No signature extracted from update response"),
        r => panic!("Unexpected probe response: {:?}", r),
    }
    mocks.iter().for_each(Mock::assert);
}

#[async_std::test]
async fn probe_response_with_extra_poll() {
    use sdk::api::ProbeResponse;
    let (url, mocks) = create_mock_server(FakeServer::ExtraPoll);
    let response = sdk::Client::new(&url).probe(0, FakeMetadata::new().get()).await.unwrap();
    match response {
        ProbeResponse::ExtraPoll(n) => assert_eq!(n, 10),
        r => panic!("Unexpected probe response: {:?}", r),
    }
    mocks.iter().for_each(Mock::assert);
}

#[async_std::test]
async fn report_success() {
    let (url, mocks) = create_mock_server(FakeServer::ReportSuccess);
    sdk::Client::new(&url)
        .report("state", FakeMetadata::new().get(), "package-uid", None, None, None)
        .await
        .unwrap();
    mocks.iter().for_each(Mock::assert);
}

#[async_std::test]
async fn report_error() {
    let (url, mocks) = create_mock_server(FakeServer::ReportError);
    sdk::Client::new(&url)
        .report(
            "state",
            FakeMetadata::new().get(),
            "package-uid",
            Some("previous-state"),
            Some("errorMessage".into()),
            None,
        )
        .await
        .unwrap();
    mocks.iter().for_each(Mock::assert);
}

#[async_std::test]
async fn download_object() {
    use async_std::fs;

    let (url, mocks) = create_mock_server(FakeServer::DownloadInParts);
    let dir = tempfile::tempdir().unwrap();
    let file_path = dir.path().join("object");

    // Download the object.
    sdk::Client::new(&url)
        .download_object(FakeMetadata::PRODUCT_UID, "package_id", dir.path(), "object")
        .await
        .unwrap();

    // Verify it has been downloaded successfully.
    assert_eq!(fs::read_to_string(&file_path).await.unwrap(), "1234".to_string());

    // Download the remaining bytes of the object.
    sdk::Client::new(&url)
        .download_object(FakeMetadata::PRODUCT_UID, "package_id", dir.path(), "object")
        .await
        .unwrap();

    // Verify it has been fully downloaded.
    assert_eq!(fs::read_to_string(&file_path).await.unwrap(), "1234567890".to_string());
    mocks.iter().for_each(Mock::assert);
    dir.close().unwrap();
}
