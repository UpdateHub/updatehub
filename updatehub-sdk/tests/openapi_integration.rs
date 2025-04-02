// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use std::collections::BTreeMap;
use testcontainers::{Container, Image, ImageArgs, clients::Cli, core::WaitFor};
use updatehub_sdk as sdk;

struct MockServer {
    docker: Cli,
}

#[derive(Debug)]
struct ApiSprout {
    volumes: BTreeMap<String, String>,
}

impl Default for ApiSprout {
    fn default() -> Self {
        let mut volumes = BTreeMap::new();
        volumes.insert(
            format!("{}/../doc/agent-http.yaml", env!("CARGO_MANIFEST_DIR")),
            "/api.yaml".to_owned(),
        );
        ApiSprout { volumes }
    }
}

#[derive(Debug, Clone, Default)]
struct ApiSproutArgs;

impl ImageArgs for ApiSproutArgs {
    fn into_iterator(self) -> Box<dyn Iterator<Item = String>> {
        let args = vec!["/api.yaml".to_string(), "--validate-request".to_string()];
        Box::new(args.into_iter())
    }
}

impl Image for ApiSprout {
    type Args = ApiSproutArgs;

    fn name(&self) -> String {
        "danielgtaylor/apisprout".to_owned()
    }

    fn tag(&self) -> String {
        "latest".to_owned()
    }

    fn ready_conditions(&self) -> Vec<WaitFor> {
        vec![WaitFor::message_on_stdout("Sprouting UpdateHub Agent local HTTP API routes on port")]
    }

    fn volumes(&self) -> Box<dyn Iterator<Item = (&String, &String)> + '_> {
        Box::new(self.volumes.iter())
    }
}

impl MockServer {
    fn new() -> Self {
        MockServer { docker: Cli::default() }
    }

    fn start(&self) -> (String, Container<ApiSprout>) {
        let apisprout = ApiSprout::default();
        let container = self.docker.run(apisprout);
        let address = format!("localhost:{}", container.get_host_port_ipv4(8000));
        (address, container)
    }
}

#[tokio::test]
async fn info() {
    let mock = MockServer::new();
    let (addr, _guard) = &mock.start();
    let client = sdk::Client::new(addr);
    let response = client.info().await;
    assert!(dbg!(response).is_ok());
}

#[tokio::test]
async fn probe_default() {
    let mock = MockServer::new();
    let (addr, _guard) = &mock.start();
    let client = sdk::Client::new(addr);
    let response = client.probe(None).await;
    match dbg!(response) {
        Ok(_) => {}
        Err(sdk::Error::AgentIsBusy(_)) => {}
        Err(e) => panic!("Unexpected Error response: {e}"),
    }
}

#[tokio::test]
async fn probe_custom() {
    let mock = MockServer::new();
    let (addr, _guard) = &mock.start();
    let client = sdk::Client::new(addr);
    let response = client.probe(Some(String::from("http://foo.bar"))).await;
    match dbg!(response) {
        Ok(_) => {}
        Err(sdk::Error::AgentIsBusy(_)) => {}
        Err(e) => panic!("Unexpected Error response: {e}"),
    }
}

#[tokio::test]
async fn local_install() {
    let mock = MockServer::new();
    let (addr, _guard) = &mock.start();
    let client = sdk::Client::new(addr);
    let file = tempfile::NamedTempFile::new().unwrap();
    let response = client.local_install(file.path()).await;

    match dbg!(response) {
        Ok(_) => {}
        Err(sdk::Error::AgentIsBusy(_)) => {}
        Err(e) => panic!("Unexpected Error response: {e}"),
    }
}

#[tokio::test]
async fn remote_install() {
    let mock = MockServer::new();
    let (addr, _guard) = &mock.start();
    let client = sdk::Client::new(addr);
    let response = client.remote_install("http://foo.bar").await;
    match dbg!(response) {
        Ok(_) => {}
        Err(sdk::Error::AgentIsBusy(_)) => {}
        Err(e) => panic!("Unexpected Error response: {e}"),
    }
}

#[tokio::test]
async fn abort_download() {
    let mock = MockServer::new();
    let (addr, _guard) = &mock.start();
    let client = sdk::Client::new(addr);
    let response = client.abort_download().await;
    match dbg!(response) {
        Ok(_) => {}
        Err(sdk::Error::AbortDownloadRefused(_)) => {}
        Err(e) => panic!("Unexpected Error response: {e}"),
    }
}

#[tokio::test]
async fn log() {
    let mock = MockServer::new();
    let (addr, _guard) = &mock.start();
    let client = sdk::Client::new(addr);
    let response = client.log().await;
    assert!(dbg!(response).is_ok());
}
