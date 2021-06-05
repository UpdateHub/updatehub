// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use testcontainers::{
    clients::Cli,
    images::generic::{GenericImage, WaitFor},
    Container, Docker, Image,
};
use updatehub_sdk as sdk;

struct MockServer {
    docker: Cli,
}

impl MockServer {
    fn new() -> Self {
        MockServer { docker: Cli::default() }
    }

    fn start(&self) -> (String, Container<Cli, GenericImage>) {
        let apisprout = GenericImage::new("danielgtaylor/apisprout:latest")
            .with_wait_for(WaitFor::message_on_stdout(
                "Sprouting UpdateHub Agent local HTTP API routes on port",
            ))
            .with_args(vec!["/api.yaml".to_string(), "--validate-request".to_string()])
            .with_volume(
                &format!("{}/../doc/agent-http.yaml", env!("CARGO_MANIFEST_DIR")),
                "/api.yaml",
            );
        let container = self.docker.run(apisprout);
        let address = format!("localhost:{}", container.get_host_port(8000).unwrap());
        (address, container)
    }
}

#[async_std::test]
async fn info() {
    let mock = MockServer::new();
    let (addr, _guard) = &mock.start();
    let client = sdk::Client::new(addr);
    let response = client.info().await;
    assert!(dbg!(response).is_ok());
}

#[async_std::test]
async fn probe_default() {
    let mock = MockServer::new();
    let (addr, _guard) = &mock.start();
    let client = sdk::Client::new(addr);
    let response = client.probe(None).await;
    match dbg!(response) {
        Ok(_) => {}
        Err(sdk::Error::AgentIsBusy(_)) => {}
        Err(e) => panic!("Unexpected Error response: {}", e),
    }
}

#[async_std::test]
async fn probe_custom() {
    let mock = MockServer::new();
    let (addr, _guard) = &mock.start();
    let client = sdk::Client::new(addr);
    let response = client.probe(Some(String::from("http://foo.bar"))).await;
    match dbg!(response) {
        Ok(_) => {}
        Err(sdk::Error::AgentIsBusy(_)) => {}
        Err(e) => panic!("Unexpected Error response: {}", e),
    }
}

#[async_std::test]
async fn local_install() {
    let mock = MockServer::new();
    let (addr, _guard) = &mock.start();
    let client = sdk::Client::new(addr);
    let file = tempfile::NamedTempFile::new().unwrap();
    let response = client.local_install(file.path()).await;

    match dbg!(response) {
        Ok(_) => {}
        Err(sdk::Error::AgentIsBusy(_)) => {}
        Err(e) => panic!("Unexpected Error response: {}", e),
    }
}

#[async_std::test]
async fn remote_install() {
    let mock = MockServer::new();
    let (addr, _guard) = &mock.start();
    let client = sdk::Client::new(addr);
    let response = client.remote_install("http://foo.bar").await;
    match dbg!(response) {
        Ok(_) => {}
        Err(sdk::Error::AgentIsBusy(_)) => {}
        Err(e) => panic!("Unexpected Error response: {}", e),
    }
}

#[async_std::test]
async fn abort_download() {
    let mock = MockServer::new();
    let (addr, _guard) = &mock.start();
    let client = sdk::Client::new(addr);
    let response = client.abort_download().await;
    match dbg!(response) {
        Ok(_) => {}
        Err(sdk::Error::AbortDownloadRefused(_)) => {}
        Err(e) => panic!("Unexpected Error response: {}", e),
    }
}

#[async_std::test]
async fn log() {
    let mock = MockServer::new();
    let (addr, _guard) = &mock.start();
    let client = sdk::Client::new(addr);
    let response = client.log().await;
    assert!(dbg!(response).is_ok());
}
