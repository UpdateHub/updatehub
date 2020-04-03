// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::{api, Error, Result};
use awc::{
    http::{
        header::{self, HeaderName, CONTENT_TYPE, RANGE, USER_AGENT},
        StatusCode,
    },
    ClientBuilder,
};
use serde::Serialize;
use slog_scope::debug;
use std::{
    convert::{TryFrom, TryInto},
    path::Path,
    time::Duration,
};
use tokio::{
    io::{self, AsyncWriteExt},
    stream::StreamExt,
};

pub struct Client<'a> {
    client: awc::Client,
    server: &'a str,
}

impl From<awc::error::SendRequestError> for Error {
    fn from(err: awc::error::SendRequestError) -> Self {
        if let awc::error::SendRequestError::Http(err) = err {
            return Error::Http(err);
        }
        Error::SendRequestError(format!("{}", err))
    }
}

pub async fn get<W>(url: &str, handle: &mut W) -> Result<()>
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

impl<'a> Client<'a> {
    pub fn new(server: &'a str) -> Self {
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
        num_retries: u64,
        firmware: api::FirmwareMetadata<'_>,
    ) -> Result<api::ProbeResponse> {
        let mut response = self
            .client
            .post(&format!("{}/upgrades", &self.server))
            .header(HeaderName::from_static("api-retries"), num_retries)
            .send_json(&firmware)
            .await?;

        match response.status() {
            StatusCode::NOT_FOUND => Ok(api::ProbeResponse::NoUpdate),
            StatusCode::OK => {
                match response
                    .headers()
                    .get("add-extra-poll")
                    .and_then(|extra_poll| extra_poll.to_str().ok())
                    .and_then(|extra_poll| extra_poll.parse().ok())
                {
                    Some(extra_poll) => Ok(api::ProbeResponse::ExtraPoll(extra_poll)),
                    None => {
                        let signature = response
                            .headers()
                            .get("UH-Signature")
                            .map(TryInto::try_into)
                            .transpose()?;
                        Ok(api::ProbeResponse::Update(
                            api::UpdatePackage::parse(&response.body().await?)?,
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
        firmware: api::FirmwareMetadata<'_>,
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
            firmware: api::FirmwareMetadata<'a>,
            package_uid: &'a str,
            #[serde(skip_serializing_if = "Option::is_none")]
            previous_state: Option<&'a str>,
            #[serde(skip_serializing_if = "Option::is_none")]
            error_message: Option<String>,
            #[serde(skip_serializing_if = "Option::is_none")]
            current_log: Option<String>,
        }

        let payload =
            Payload { state, firmware, package_uid, previous_state, error_message, current_log };

        self.client.post(&format!("{}/report", &self.server)).send_json(&payload).await?;
        Ok(())
    }
}

impl TryFrom<&header::HeaderValue> for api::Signature {
    type Error = Error;

    fn try_from(value: &header::HeaderValue) -> Result<Self> {
        Ok(Self::from_base64_str(value.to_str()?)?)
    }
}
