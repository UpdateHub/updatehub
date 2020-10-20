// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::{api, Error, Result};
use std::path::Path;
use surf::{Body, StatusCode};

/// The `Client` allow for requests to be sent.
pub struct Client {
    server_address: String,
    client: surf::Client,
}

impl Clone for Client {
    fn clone(&self) -> Self {
        Client { server_address: self.server_address.clone(), client: surf::Client::new() }
    }
}

impl Default for Client {
    fn default() -> Self {
        Client { server_address: "http://localhost:8080".to_string(), client: surf::Client::new() }
    }
}

impl Client {
    /// Constructs a new `Client`.
    pub fn new(server_address: &str) -> Self {
        Client { server_address: format!("http://{}", server_address), ..Self::default() }
    }

    /// Get the current state of the agent.
    /// # Example
    ///
    /// ```no_run
    /// # async fn run() -> updatehub_sdk::Result<()> {
    /// let client = updatehub_sdk::Client::default();
    /// let response = client.info().await?;
    /// # Ok(()) }
    /// ```
    ///
    /// # Errors
    ///
    /// This method fails when cannot complete the request at the address or
    /// cannot parse the body json as a `info::Response`.
    pub async fn info(&self) -> Result<api::info::Response> {
        let mut response = self.client.get(&format!("{}/info", self.server_address)).await?;

        match response.status() {
            StatusCode::Ok => Ok(response.body_json().await?),
            s => Err(Error::UnexpectedResponse(s)),
        }
    }

    /// Probe the agent for update.
    /// # Example
    ///
    /// ```no_run
    /// # async fn run() -> updatehub_sdk::Result<()> {
    /// let client = updatehub_sdk::Client::default();
    /// let response = client.probe(None).await?;
    /// # Ok(()) }
    /// ```
    ///
    /// A **custom** address can be used:
    ///
    /// ```no_run
    /// # async fn run() -> updatehub_sdk::Result<()> {
    /// let client = updatehub_sdk::Client::default();
    /// let response = client.probe(Some("http://foo.bar".to_string())).await?;
    /// # Ok(()) }
    /// ```
    ///
    /// # Errors
    ///
    /// This method fails when cannot complete the request at the address or
    /// cannot parse the body json as a `probe::Response`.
    pub async fn probe(&self, custom: Option<String>) -> Result<api::probe::Response> {
        let request = self.client.post(&format!("{}/probe", self.server_address));
        let mut response = match custom {
            Some(custom_server) => {
                request.body(Body::from_json(&api::probe::Request { custom_server })?).await?
            }
            None => request.await?,
        };

        match response.status() {
            StatusCode::Ok => Ok(response.body_json().await?),
            StatusCode::NotAcceptable => Err(Error::AgentIsBusy(response.body_json().await?)),
            s => Err(Error::UnexpectedResponse(s)),
        }
    }

    /// Request agent to install a local update package passing a path as
    /// argument.
    /// # Example
    ///
    /// ```no_run
    /// # async fn run() -> updatehub_sdk::Result<()> {
    /// let path = std::path::Path::new("/tmp/my-update-package.uhupkg");
    ///
    /// let client = updatehub_sdk::Client::default();
    /// let response = client.local_install(path).await?;
    /// # Ok(()) }
    /// ```
    ///
    /// # Errors
    ///
    /// This method fails when cannot complete the request at the address or
    /// cannot parse the body json as a `state::Response`.
    pub async fn local_install(&self, file: &Path) -> Result<api::state::Response> {
        let mut response = self
            .client
            .post(&format!("{}/local_install", self.server_address))
            .body(Body::from_json(&api::local_install::Request { file: file.to_owned() })?)
            .await?;

        match response.status() {
            StatusCode::Ok => Ok(response.body_json().await?),
            StatusCode::NotAcceptable => Err(Error::AgentIsBusy(response.body_json().await?)),
            s => Err(Error::UnexpectedResponse(s)),
        }
    }

    /// Request agent to install a package from a URL.
    /// # Example
    ///
    /// ```no_run
    /// # async fn run() -> updatehub_sdk::Result<()> {
    /// let client = updatehub_sdk::Client::default();
    /// let response = client.remote_install("http://foo.bar").await?;
    /// # Ok(()) }
    /// ```
    ///
    /// # Errors
    ///
    /// This method fails when cannot complete the request at the address or
    /// cannot parse the body json as a `state::Response`.
    pub async fn remote_install(&self, url: &str) -> Result<api::state::Response> {
        let mut response = self
            .client
            .post(&format!("{}/remote_install", self.server_address))
            .body(Body::from_json(&api::remote_install::Request { url: url.to_owned() })?)
            .await?;

        match response.status() {
            StatusCode::Ok => Ok(response.body_json().await?),
            s => Err(Error::UnexpectedResponse(s)),
        }
    }

    /// Tells agent to abort the current download.
    /// # Example
    ///
    /// ```no_run
    /// # async fn run() -> updatehub_sdk::Result<()> {
    /// let client = updatehub_sdk::Client::default();
    /// let response = client.abort_download().await?;
    /// # Ok(()) }
    /// ```
    ///
    /// # Errors
    ///
    /// This method fails when cannot complete the request at the address or
    /// cannot parse the body json as a `state::Response`.
    pub async fn abort_download(&self) -> Result<api::state::Response> {
        let mut response =
            self.client.post(&format!("{}/update/download/abort", self.server_address)).await?;

        match response.status() {
            StatusCode::Ok => Ok(response.body_json().await?),
            StatusCode::NotAcceptable => {
                Err(Error::AbortDownloadRefused(response.body_json().await?))
            }
            s => Err(Error::UnexpectedResponse(s)),
        }
    }

    /// Get the available log entries for the last update.
    /// # Example
    ///
    /// ```no_run
    /// # async fn run() -> updatehub_sdk::Result<()> {
    /// let client = updatehub_sdk::Client::default();
    /// let response = client.log().await?;
    /// # Ok(()) }
    /// ```
    ///
    /// # Errors
    ///
    /// This method fails when cannot complete the request at the address or
    /// cannot parse the body json as a `log::Log`.
    pub async fn log(&self) -> Result<api::log::Log> {
        let mut response = self.client.get(&format!("{}/log", self.server_address)).await?;

        match response.status() {
            StatusCode::Ok => Ok(response.body_json().await?),
            s => Err(Error::UnexpectedResponse(s)),
        }
    }
}
