// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::{firmware::Metadata, runtime_settings::RuntimeSettings, update_package::UpdatePackage};
use reqwest::{
    header::{HeaderMap, HeaderName, CONTENT_TYPE, RANGE, USER_AGENT},
    Client, StatusCode,
};
use sdk::api::info::firmware as api;
use serde::Serialize;
use slog_scope::debug;
use std::{path::Path, time::Duration};
use thiserror::Error;

#[cfg(test)]
pub(crate) mod tests;

pub(crate) struct Api<'a> {
    server: &'a str,
}

#[derive(Debug)]
pub(crate) enum ProbeResponse {
    NoUpdate,
    Update(UpdatePackage),
    ExtraPoll(i64),
}

pub type Result<T> = std::result::Result<T, Error>;

#[derive(Debug, Error)]
pub enum Error {
    #[error("Invalid status code received: {0}")]
    InvalidStatusResponse(reqwest::StatusCode),

    #[error("Update package error: {0}")]
    UpdatePackage(#[from] crate::update_package::Error),

    #[error("Update package error: {0}")]
    Io(#[from] std::io::Error),

    #[error("Client error: {0}")]
    Client(#[from] reqwest::Error),

    #[error("Invalid header error: {0}")]
    InvalidHeader(#[from] reqwest::header::InvalidHeaderValue),
}

// We redefine the metadata structure here because the cloud
// uses a different serialization format than we use on
// the local sdk.
#[derive(Serialize)]
#[serde(rename_all = "kebab-case")]
struct FirmwareMetadata<'a> {
    pub product_uid: &'a str,
    pub version: &'a str,
    pub hardware: &'a str,
    pub device_identity: &'a api::MetadataValue,
    pub device_attributes: &'a api::MetadataValue,
}

impl<'a> FirmwareMetadata<'a> {
    fn from_sdk(metadata: &'a api::Metadata) -> Self {
        FirmwareMetadata {
            product_uid: &metadata.product_uid,
            version: &metadata.version,
            hardware: &metadata.hardware,
            device_identity: &metadata.device_identity,
            device_attributes: &metadata.device_attributes,
        }
    }
}

impl<'a> Api<'a> {
    pub(crate) fn new(server: &'a str) -> Self {
        Self { server }
    }

    fn client(&self) -> Result<Client> {
        let mut headers = HeaderMap::new();

        headers.insert(USER_AGENT, "updatehub/next".parse()?);
        headers.insert(CONTENT_TYPE, "application/json".parse()?);
        headers.insert(
            HeaderName::from_static("api-content-type"),
            "application/vnd.updatehub-v1+json".parse()?,
        );

        Ok(Client::builder().timeout(Duration::from_secs(10)).default_headers(headers).build()?)
    }

    pub async fn probe(
        &self,
        runtime_settings: &RuntimeSettings,
        firmware: &Metadata,
    ) -> Result<ProbeResponse> {
        let response = self
            .client()?
            .post(&format!("{}/upgrades", &self.server))
            .header(HeaderName::from_static("api-retries"), runtime_settings.retries())
            .json(&FirmwareMetadata::from_sdk(&firmware.0))
            .send()
            .await?;

        match response.status() {
            StatusCode::NOT_FOUND => Ok(ProbeResponse::NoUpdate),
            StatusCode::OK => {
                match response
                    .headers()
                    .get("add-extra-poll")
                    .and_then(|extra_poll| extra_poll.to_str().ok())
                    .and_then(|extra_poll| extra_poll.parse().ok())
                {
                    Some(extra_poll) => Ok(ProbeResponse::ExtraPoll(extra_poll)),
                    None => {
                        Ok(ProbeResponse::Update(UpdatePackage::parse(&response.bytes().await?)?))
                    }
                }
            }
            s => Err(Error::InvalidStatusResponse(s)),
        }
    }

    pub async fn download_object(
        &self,
        product_uid: &str,
        package_uid: &str,
        download_dir: &Path,
        object: &str,
    ) -> Result<()> {
        use std::fs::create_dir_all;
        use tokio::{fs::OpenOptions, io::AsyncWriteExt};

        // FIXME: Discuss the need of packages inside the route
        let mut client = self.client()?.get(&format!(
            "{}/products/{}/packages/{}/objects/{}",
            &self.server, product_uid, package_uid, object
        ));

        if !download_dir.exists() {
            debug!("Creating directory to store the downloads.");
            create_dir_all(download_dir)?;
        }

        let file = download_dir.join(object);
        if file.exists() {
            client = client
                .header(RANGE, format!("bytes={}-", file.metadata()?.len().saturating_sub(1)));
        }

        let mut file = OpenOptions::new().create(true).append(true).open(&file).await?;
        let mut response = client.send().await?;
        if response.status().is_success() {
            while let Some(chunk) = response.chunk().await? {
                file.write_all(&chunk).await?;
            }
            return Ok(());
        }

        Err(Error::InvalidStatusResponse(response.status()))
    }

    pub async fn report(
        &self,
        state: &str,
        firmware: &Metadata,
        package_uid: &str,
        previous_state: Option<&str>,
        error_message: Option<String>,
        current_log: Option<String>,
    ) -> Result<()> {
        #[derive(Serialize)]
        #[serde(rename_all = "kebab-case")]
        struct Payload<'a> {
            #[serde(rename = "status")]
            state: &'a str,
            #[serde(flatten)]
            firmware: FirmwareMetadata<'a>,
            package_uid: &'a str,
            #[serde(skip_serializing_if = "Option::is_none")]
            previous_state: Option<&'a str>,
            #[serde(skip_serializing_if = "Option::is_none")]
            error_message: Option<String>,
            #[serde(skip_serializing_if = "Option::is_none")]
            current_log: Option<String>,
        }

        let firmware = FirmwareMetadata::from_sdk(&firmware.0);
        let payload =
            Payload { state, firmware, package_uid, previous_state, error_message, current_log };

        self.client()?.post(&format!("{}/report", &self.server)).json(&payload).send().await?;
        Ok(())
    }
}
