// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::{api, Result};
use std::path::PathBuf;

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
            _ => Err(response.into()),
        }
    }

    pub async fn probe(&self, custom: Option<String>) -> Result<api::info::Response> {
        let request = self.client.post(&format!("{}/probe", self.server_address));
        let response = match custom {
            Some(custom) => request.body(custom),
            None => request,
        }
        .send()
        .await?;

        match response.status() {
            reqwest::StatusCode::OK => Ok(response.json().await?),
            reqwest::StatusCode::ACCEPTED => {
                Err(response.json::<api::state::Response>().await?.into())
            }
            _ => Err(response.into()),
        }
    }

    pub async fn local_install(&self, file: PathBuf) -> Result<api::info::Response> {
        let response = self
            .client
            .post(&format!("{}/local_install", self.server_address))
            .body(format!("{:?}", file))
            .send()
            .await?;

        match response.status() {
            reqwest::StatusCode::OK => Ok(response.json().await?),
            reqwest::StatusCode::UNPROCESSABLE_ENTITY => {
                Err(response.json::<api::state::Response>().await?.into())
            }
            _ => Err(response.into()),
        }
    }

    pub async fn remote_install(&self, url: String) -> Result<api::info::Response> {
        let response = self
            .client
            .post(&format!("{}/remote_install", self.server_address))
            .body(url)
            .send()
            .await?;

        match response.status() {
            reqwest::StatusCode::OK => Ok(response.json().await?),
            reqwest::StatusCode::UNPROCESSABLE_ENTITY => {
                Err(response.json::<api::state::Response>().await?.into())
            }
            _ => Err(response.into()),
        }
    }

    pub async fn abort_download(&self) -> Result<api::info::Response> {
        let response = self
            .client
            .post(&format!("{}/update/download/abort", self.server_address))
            .send()
            .await?;

        match response.status() {
            reqwest::StatusCode::OK => Ok(response.json().await?),
            reqwest::StatusCode::BAD_REQUEST => {
                Err(response.json::<api::abort_download::Refused>().await?.into())
            }
            _ => Err(response.into()),
        }
    }

    pub async fn log(&self) -> Result<Vec<api::log::Entry>> {
        let response = self.client.get(&format!("{}/log", self.server_address)).send().await?;

        match response.status() {
            reqwest::StatusCode::OK => Ok(response.json().await?),
            _ => Err(response.into()),
        }
    }
}
