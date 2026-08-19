// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use testcontainers::{
    ContainerAsync, GenericImage, ImageExt,
    core::{Mount, WaitFor},
    runners::AsyncRunner,
};
use updatehub_sdk as sdk;

/// Starts an apisprout container serving the agent's OpenAPI document, so the
/// SDK is exercised against the very schema it is supposed to implement.
async fn start_mock_server() -> (String, ContainerAsync<GenericImage>) {
    let container = GenericImage::new("danielgtaylor/apisprout", "latest")
        .with_wait_for(WaitFor::message_on_stdout(
            "Sprouting UpdateHub Agent local HTTP API routes on port",
        ))
        .with_mount(Mount::bind_mount(
            format!("{}/../doc/agent-http.yaml", env!("CARGO_MANIFEST_DIR")),
            "/api.yaml",
        ))
        .with_cmd(["/api.yaml", "--validate-request"])
        .start()
        .await
        .expect("failed to start the apisprout container");
    let port = container.get_host_port_ipv4(8000).await.expect("failed to get the mapped port");

    (format!("localhost:{port}"), container)
}

#[tokio::test]
async fn info() {
    let (addr, _guard) = start_mock_server().await;
    let client = sdk::Client::new(&addr);
    let response = client.info().await;
    assert!(dbg!(response).is_ok());
}

#[tokio::test]
async fn probe_default() {
    let (addr, _guard) = start_mock_server().await;
    let client = sdk::Client::new(&addr);
    let response = client.probe(None).await;
    match dbg!(response) {
        Ok(_) | Err(sdk::Error::AgentIsBusy(_)) => {}
        Err(e) => panic!("Unexpected Error response: {e}"),
    }
}

#[tokio::test]
async fn probe_custom() {
    let (addr, _guard) = start_mock_server().await;
    let client = sdk::Client::new(&addr);
    let response = client.probe(Some(String::from("http://foo.bar"))).await;
    match dbg!(response) {
        Ok(_) | Err(sdk::Error::AgentIsBusy(_)) => {}
        Err(e) => panic!("Unexpected Error response: {e}"),
    }
}

#[tokio::test]
async fn local_install() {
    let (addr, _guard) = start_mock_server().await;
    let client = sdk::Client::new(&addr);
    let file = tempfile::NamedTempFile::new().unwrap();
    let response = client.local_install(file.path()).await;

    match dbg!(response) {
        Ok(_) | Err(sdk::Error::AgentIsBusy(_)) => {}
        Err(e) => panic!("Unexpected Error response: {e}"),
    }
}

#[tokio::test]
async fn remote_install() {
    let (addr, _guard) = start_mock_server().await;
    let client = sdk::Client::new(&addr);
    let response = client.remote_install("http://foo.bar").await;
    match dbg!(response) {
        Ok(_) | Err(sdk::Error::AgentIsBusy(_)) => {}
        Err(e) => panic!("Unexpected Error response: {e}"),
    }
}

#[tokio::test]
async fn abort_download() {
    let (addr, _guard) = start_mock_server().await;
    let client = sdk::Client::new(&addr);
    let response = client.abort_download().await;
    match dbg!(response) {
        Ok(_) | Err(sdk::Error::AbortDownloadRefused(_)) => {}
        Err(e) => panic!("Unexpected Error response: {e}"),
    }
}

#[tokio::test]
async fn log() {
    let (addr, _guard) = start_mock_server().await;
    let client = sdk::Client::new(&addr);
    let response = client.log().await;
    assert!(dbg!(response).is_ok());
}
