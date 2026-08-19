// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::{Error, Result, api};
use reqwest::{StatusCode, header};
use slog_scope::{debug, error};
use std::{
    convert::{TryFrom, TryInto},
    path::Path,
};
use tokio::{fs, io};

pub struct Client<'a> {
    client: reqwest::Client,
    server: &'a str,
}

/// Downloads the content of `url` into `handle`.
///
/// # Errors
///
/// Returns an error when `url` does not parse, when the request fails, when the
/// server answers with a status other than success, or when the write to
/// `handle` fails.
pub async fn get<W>(url: &str, handle: &mut W) -> Result<()>
where
    W: io::AsyncWrite + Unpin,
{
    let url = reqwest::Url::parse(url)?;
    save_body_to(reqwest::get(url).await?, handle).await
}

async fn save_body_to<W>(mut resp: reqwest::Response, handle: &mut W) -> Result<()>
where
    W: io::AsyncWrite + Unpin,
{
    use io::AsyncWriteExt;
    use std::str::FromStr;

    if !resp.status().is_success() {
        return Err(Error::InvalidStatusResponse(resp.status()));
    }

    let mut written: u64 = 0;
    let mut threshold = 10;
    let length = match resp.headers().get(header::CONTENT_LENGTH) {
        Some(v) => u64::from_str(v.to_str()?)?,
        None => 0,
    };

    while let Some(chunk) = resp.chunk().await? {
        let read = chunk.len() as u64;
        handle.write_all(&chunk).await?;
        if length > 0 {
            written += read;
            let percent = written * 100 / length;
            if percent >= threshold {
                threshold += 20;
                debug!("{}% of the file has been downloaded", std::cmp::min(percent, 100));
            }
        }
    }
    handle.flush().await?;

    Ok(())
}

impl<'a> Client<'a> {
    /// Constructs a client that talks to the server at `server`.
    ///
    /// # Panics
    ///
    /// Panics when the platform gives no TLS backend to build the HTTP client
    /// with.
    #[must_use]
    pub fn new(server: &'a str) -> Self {
        let mut headers = header::HeaderMap::new();
        headers.insert(header::USER_AGENT, header::HeaderValue::from_static("updatehub/2.0 Linux"));
        headers.insert(header::CONTENT_TYPE, header::HeaderValue::from_static("application/json"));
        headers.insert(
            "api-content-type",
            header::HeaderValue::from_static("application/vnd.updatehub-v1+json"),
        );

        let client = reqwest::Client::builder()
            .connect_timeout(std::time::Duration::from_secs(10))
            .default_headers(headers)
            .build()
            .unwrap();

        Self { client, server }
    }

    /// Asks the server whether an update is available for this device.
    ///
    /// # Errors
    ///
    /// Returns an error when the server address does not parse, when the
    /// request fails, when the server answers with an unexpected status, or
    /// when the update metadata does not parse.
    pub async fn probe(
        &self,
        num_retries: usize,
        firmware: api::FirmwareMetadata<'_>,
    ) -> Result<api::ProbeResponse> {
        reqwest::Url::parse(self.server)?;

        let response = self
            .client
            .post(format!("{}/upgrades", &self.server))
            .header("api-retries", num_retries.to_string())
            .json(&firmware)
            .send()
            .await?;

        match response.status() {
            StatusCode::NOT_FOUND => Ok(api::ProbeResponse::NoUpdate),
            StatusCode::OK => {
                let extra_poll = response
                    .headers()
                    .get("add-extra-poll")
                    .and_then(|extra_poll| extra_poll.to_str().ok())
                    .and_then(|extra_poll| extra_poll.parse().ok());

                if let Some(extra_poll) = extra_poll {
                    Ok(api::ProbeResponse::ExtraPoll(extra_poll))
                } else {
                    let signature = response
                        .headers()
                        .get("UH-Signature")
                        .map(TryInto::try_into)
                        .transpose()?;
                    Ok(api::ProbeResponse::Update(
                        api::UpdatePackage::parse(&response.bytes().await?)?,
                        signature,
                    ))
                }
            }
            s => Err(Error::InvalidStatusResponse(s)),
        }
    }

    /// Downloads one object of an update package into `download_dir`.
    ///
    /// Downloads that stopped part way continue from the number of bytes
    /// already on disk.
    ///
    /// # Errors
    ///
    /// Returns an error when the server address does not parse, when the
    /// request fails, when the server answers with a status other than
    /// success, or when the write to disk fails.
    pub async fn download_object(
        &self,
        product_uid: &str,
        package_uid: &str,
        download_dir: &Path,
        object: &str,
    ) -> Result<()> {
        validate_url(self.server)?;

        // FIXME: Discuss the need of packages inside the route
        let mut request = self.client.get(format!(
            "{}/products/{}/packages/{}/objects/{}",
            &self.server, product_uid, package_uid, object
        ));

        if !download_dir.exists() {
            fs::create_dir_all(download_dir).await.map_err(|e| {
                error!("fail to create {:?} directory, error: {}", download_dir, e);
                e
            })?;
        }

        let file = download_dir.join(object);
        if file.exists() {
            request = request
                .header("RANGE", format!("bytes={}-", file.metadata()?.len().saturating_sub(1)));
        }

        let mut file = fs::OpenOptions::new().create(true).append(true).open(&file).await?;

        save_body_to(request.send().await?, &mut file).await
    }

    /// Reports the current state of the device to the server.
    ///
    /// # Errors
    ///
    /// Returns an error when the server address does not parse or when the
    /// request fails.
    pub async fn report(
        &self,
        state: &str,
        firmware: api::FirmwareMetadata<'_>,
        package_uid: &str,
        previous_state: Option<&str>,
        error_message: Option<String>,
        current_log: Option<String>,
    ) -> Result<()> {
        #[derive(serde::Serialize)]
        #[serde(rename_all = "kebab-case")]
        struct Payload<'a> {
            #[serde(rename = "status")]
            state: &'a str,
            #[serde(flatten)]
            firmware: api::FirmwareMetadata<'a>,
            package_uid: &'a str,
            #[serde(skip_serializing_if = "Option::is_none")]
            previous_state: Option<&'a str>,
            #[serde(skip_serializing_if = "Option::is_none")]
            error_message: Option<String>,
            #[serde(skip_serializing_if = "Option::is_none")]
            current_log: Option<String>,
        }

        validate_url(self.server)?;

        let payload =
            Payload { state, firmware, package_uid, previous_state, error_message, current_log };

        self.client.post(format!("{}/report", &self.server)).json(&payload).send().await?;
        Ok(())
    }
}

impl TryFrom<&header::HeaderValue> for api::Signature {
    type Error = Error;

    fn try_from(value: &header::HeaderValue) -> Result<Self> {
        let value = value.to_str()?;

        // Workarround for https://github.com/sfackler/rust-openssl/issues/1325
        if value.is_empty() {
            return Self::from_base64_str("");
        }

        Self::from_base64_str(value)
    }
}

fn validate_url(url: &str) -> Result<()> {
    url::Url::parse(url)?;
    Ok(())
}
