// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::{api, error::Result};

pub struct Request<'a> {
    pub(super) server_address: &'a str,
    pub(super) client: &'a reqwest::Client,
}

impl<'a> Request<'a> {
    pub fn get(&self) -> InfoRequest {
        InfoRequest { request: self.client.get(&format!("{}/info", self.server_address)) }
    }
}

pub struct InfoRequest {
    request: reqwest::RequestBuilder,
}

impl InfoRequest {
    pub async fn send(self) -> Result<api::info::Info> {
        let response = self.request.send().await?;

        match response.status() {
            reqwest::StatusCode::OK => Ok(response.json().await?),
            _ => panic!("Error"),
        }
    }
}
