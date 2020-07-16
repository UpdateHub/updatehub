// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::{api, Error, Result};
use awc::http::StatusCode;
use std::path::Path;

#[derive(Clone)]
pub struct Client {
    server_address: String,
    client: awc::Client,
}

impl Default for Client {
    fn default() -> Self {
        Client { server_address: "http://localhost:8080".to_string(), client: awc::Client::new() }
    }
}

impl Client {
    pub fn new(server_address: &str) -> Self {
        Client { server_address: format!("http://{}", server_address), ..Self::default() }
    }

    pub async fn info(&self) -> Result<api::info::Response> {
        let mut response = self.client.get(&format!("{}/info", self.server_address)).send().await?;

        match response.status() {
            StatusCode::OK => Ok(response.json().await?),
            s => Err(Error::UnexpectedResponse(s)),
        }
    }

    pub async fn probe(&self, custom: Option<String>) -> Result<api::probe::Response> {
        let request = self.client.post(&format!("{}/probe", self.server_address));
        let mut response = match custom {
            Some(custom_server) => request.send_json(&api::probe::Request { custom_server }),
            None => request.send(),
        }
        .await?;

        match response.status() {
            StatusCode::OK => Ok(response.json().await?),
            StatusCode::ACCEPTED => {
                Err(Error::AgentIsBusy(response.json::<api::state::Response>().await?))
            }
            s => Err(Error::UnexpectedResponse(s)),
        }
    }

    pub async fn local_install(&self, file: &Path) -> Result<api::state::Response> {
        let mut response = self
            .client
            .post(&format!("{}/local_install", self.server_address))
            .send_json(&api::local_install::Request { file: file.to_owned() })
            .await?;

        match response.status() {
            StatusCode::OK => Ok(response.json().await?),
            StatusCode::UNPROCESSABLE_ENTITY => {
                Err(Error::AgentIsBusy(response.json::<api::state::Response>().await?))
            }
            s => Err(Error::UnexpectedResponse(s)),
        }
    }

    pub async fn remote_install(&self, url: &str) -> Result<api::state::Response> {
        let mut response = self
            .client
            .post(&format!("{}/remote_install", self.server_address))
            .send_json(&api::remote_install::Request { url: url.to_owned() })
            .await?;

        match response.status() {
            StatusCode::OK => Ok(response.json().await?),
            s => Err(Error::UnexpectedResponse(s)),
        }
    }

    pub async fn abort_download(&self) -> Result<api::abort_download::Response> {
        let mut response = self
            .client
            .post(&format!("{}/update/download/abort", self.server_address))
            .send()
            .await?;

        match response.status() {
            StatusCode::OK => Ok(response.json().await?),
            StatusCode::BAD_REQUEST => Err(Error::AbortDownloadRefused(
                response.json::<api::abort_download::Refused>().await?,
            )),
            s => Err(Error::UnexpectedResponse(s)),
        }
    }

    pub async fn log(&self) -> Result<Vec<api::log::Entry>> {
        let mut response = self.client.get(&format!("{}/log", self.server_address)).send().await?;

        match response.status() {
            StatusCode::OK => Ok(response.json().await?),
            s => Err(Error::UnexpectedResponse(s)),
        }
    }
}
