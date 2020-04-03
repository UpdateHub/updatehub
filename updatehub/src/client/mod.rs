// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

#[cfg(test)]
pub(crate) mod tests;

use crate::{
    firmware::Metadata,
    runtime_settings::RuntimeSettings,
    update_package::{Signature, UpdatePackage},
};
use awc::{
    http::{
        header::{self, HeaderName, CONTENT_TYPE, RANGE, USER_AGENT},
        StatusCode,
    },
    Client, ClientBuilder,
};
use sdk::api::info::firmware as api;
use serde::Serialize;
use slog_scope::debug;
use std::{
    convert::{TryFrom, TryInto},
    path::Path,
    time::Duration,
};
use thiserror::Error;
use tokio::{
    io::{self, AsyncWriteExt},
    stream::StreamExt,
};

pub(crate) struct Api<'a> {
    client: Client,
    server: &'a str,
}

#[derive(Debug)]
pub(crate) enum ProbeResponse {
    NoUpdate,
    Update(UpdatePackage, Option<Signature>),
    ExtraPoll(i64),
}

pub type Result<T> = std::result::Result<T, Error>;

#[derive(Debug, Error)]
pub enum Error {
    #[error("Http response is missing Content Length")]
    MissingContentLength,

    #[error(transparent)]
    ParseInt(#[from] std::num::ParseIntError),

    #[error("Invalid status code received: {0}")]
    InvalidStatusResponse(StatusCode),

    #[error("Update package error: {0}")]
    UpdatePackage(#[from] crate::update_package::Error),

    #[error("Update package error: {0}")]
    Io(#[from] std::io::Error),

    #[error(transparent)]
    ConnectError(#[from] awc::error::ConnectError),

    #[error("Send Request Error: {0}")]
    SendRequestError(String),

    #[error(transparent)]
    Http(awc::error::HttpError),

    #[error(transparent)]
    PayloadError(#[from] awc::error::PayloadError),

    #[error(transparent)]
    JsonPayloadError(#[from] awc::error::JsonPayloadError),

    #[error("Invalid header error: {0}")]
    InvalidHeader(#[from] awc::http::header::InvalidHeaderValue),

    #[error("Non str header error: {0}")]
    NonStrHeader(#[from] awc::http::header::ToStrError),
}

impl From<awc::error::SendRequestError> for Error {
    fn from(err: awc::error::SendRequestError) -> Self {
        if let awc::error::SendRequestError::Http(err) = err {
            return Error::Http(err);
        }
        Error::SendRequestError(format!("{}", err))
    }
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

pub(crate) async fn get<W>(url: &str, handle: &mut W) -> Result<()>
where
    W: io::AsyncWrite + Unpin,
{
    let req = awc::Client::new().get(url);
    save_body_to(req, handle).await
}

async fn save_body_to<W>(req: awc::ClientRequest, handle: &mut W) -> Result<()>
where
    W: io::AsyncWrite + Unpin,
{
    use std::str::FromStr;

    let mut rep = req.send().await?;
    if !rep.status().is_success() {
        return Err(Error::InvalidStatusResponse(rep.status()));
    }
    let length = usize::from_str(
        rep.headers()
            .get(header::CONTENT_LENGTH)
            .ok_or_else(|| Error::MissingContentLength)?
            .to_str()?,
    )?;
    let mut written: f32 = 0.;
    let mut threshold = 10;

    while let Some(chunk) = rep.next().await {
        let chunk = &chunk?;
        handle.write_all(&chunk).await?;
        written += chunk.len() as f32 / (length / 100) as f32;
        if written as usize >= threshold {
            threshold += 20;
            debug!("{}% of the file has been downloaded", written as usize);
        }
    }
    debug!("100% of the file has been downloaded");

    Ok(())
}

impl<'a> Api<'a> {
    pub(crate) fn new(server: &'a str) -> Self {
        let client = ClientBuilder::new()
            .timeout(Duration::from_secs(10))
            .header(USER_AGENT, "updatehub/next")
            .header(CONTENT_TYPE, "application/json")
            .header(
                HeaderName::from_static("api-content-type"),
                "application/vnd.updatehub-v1+json",
            )
            .finish();
        Self { server, client }
    }

    pub async fn probe(
        &self,
        runtime_settings: &RuntimeSettings,
        firmware: &Metadata,
    ) -> Result<ProbeResponse> {
        let mut response = self
            .client
            .post(&format!("{}/upgrades", &self.server))
            .header(HeaderName::from_static("api-retries"), runtime_settings.retries())
            .send_json(&FirmwareMetadata::from_sdk(&firmware.0))
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
                        let signature = response
                            .headers()
                            .get("UH-Signature")
                            .map(TryInto::try_into)
                            .transpose()?;
                        Ok(ProbeResponse::Update(
                            UpdatePackage::parse(&response.body().await?)?,
                            signature,
                        ))
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
        use tokio::fs::{create_dir_all, OpenOptions};

        // FIXME: Discuss the need of packages inside the route
        let mut request = self.client.get(&format!(
            "{}/products/{}/packages/{}/objects/{}",
            &self.server, product_uid, package_uid, object
        ));

        if !download_dir.exists() {
            debug!("Creating directory to store the downloads.");
            create_dir_all(download_dir).await?;
        }

        let file = download_dir.join(object);
        if file.exists() {
            request = request
                .header(RANGE, format!("bytes={}-", file.metadata()?.len().saturating_sub(1)));
        }

        let mut file = OpenOptions::new().create(true).append(true).open(&file).await?;

        save_body_to(request, &mut file).await
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

        self.client.post(&format!("{}/report", &self.server)).send_json(&payload).await?;
        Ok(())
    }
}

impl TryFrom<&header::HeaderValue> for Signature {
    type Error = Error;

    fn try_from(value: &header::HeaderValue) -> Result<Self> {
        Ok(Self::from_str(value.to_str()?)?)
    }
}
