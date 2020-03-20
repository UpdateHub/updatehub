// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::{api, Error, Result};
use std::path::Path;

#[derive(Clone)]
pub struct Client {
    server_address: String,
    client: reqwest::Client,
}

impl Default for Client {
    fn default() -> Self {
        Client {
            server_address: "http://localhost:8080".to_string(),
            client: reqwest::Client::new(),
        }
    }
}

impl Client {
    pub fn new(server_address: &str) -> Self {
        Client { server_address: format!("http://{}", server_address), ..Self::default() }
    }

    pub async fn info(&self) -> Result<api::info::Response> {
        let response = self.client.get(&format!("{}/info", self.server_address)).send().await?;

        match response.status() {
            reqwest::StatusCode::OK => Ok(response.json().await?),
            _ => Err(Error::UnexpectedResponse(response)),
        }
    }

    pub async fn probe(&self, custom: Option<String>) -> Result<api::probe::Response> {
        let request = self.client.post(&format!("{}/probe", self.server_address));
        let response = match custom {
            Some(custom_server) => request.json(&api::probe::Request { custom_server }),
            None => request,
        }
        .send()
        .await?;

        match response.status() {
            reqwest::StatusCode::OK => Ok(response.json().await?),
            reqwest::StatusCode::ACCEPTED => {
                Err(Error::AgentIsBusy(response.json::<api::state::Response>().await?))
            }
            _ => Err(Error::UnexpectedResponse(response)),
        }
    }

    pub async fn local_install(&self, file: &Path) -> Result<api::state::Response> {
        let response = self
            .client
            .post(&format!("{}/local_install", self.server_address))
            .header(reqwest::header::CONTENT_TYPE, "text/plain")
            .body(format!("{}", file.display()))
            .send()
            .await?;

        match response.status() {
            reqwest::StatusCode::OK => Ok(response.json().await?),
            reqwest::StatusCode::UNPROCESSABLE_ENTITY => {
                Err(Error::AgentIsBusy(response.json::<api::state::Response>().await?))
            }
            _ => Err(Error::UnexpectedResponse(response)),
        }
    }

    pub async fn remote_install(&self, url: String) -> Result<api::state::Response> {
        let response = self
            .client
            .post(&format!("{}/remote_install", self.server_address))
            .header(reqwest::header::CONTENT_TYPE, "text/plain")
            .body(url)
            .send()
            .await?;

        match response.status() {
            reqwest::StatusCode::OK => Ok(response.json().await?),
            reqwest::StatusCode::UNPROCESSABLE_ENTITY => {
                Err(Error::AgentIsBusy(response.json::<api::state::Response>().await?))
            }
            _ => Err(Error::UnexpectedResponse(response)),
        }
    }

    pub async fn abort_download(&self) -> Result<api::abort_download::Response> {
        let response = self
            .client
            .post(&format!("{}/update/download/abort", self.server_address))
            .send()
            .await?;

        match response.status() {
            reqwest::StatusCode::OK => Ok(response.json().await?),
            reqwest::StatusCode::BAD_REQUEST => Err(Error::AbortDownloadRefused(
                response.json::<api::abort_download::Refused>().await?,
            )),
            _ => Err(Error::UnexpectedResponse(response)),
        }
    }

    pub async fn log(&self) -> Result<Vec<api::log::Entry>> {
        let response = self.client.get(&format!("{}/log", self.server_address)).send().await?;

        match response.status() {
            reqwest::StatusCode::OK => Ok(response.json().await?),
            _ => Err(Error::UnexpectedResponse(response)),
        }
    }
}
