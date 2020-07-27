// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::{api, Error, Result};
use async_std::{fs, io};
use slog_scope::{debug, error};
use std::{
    convert::{TryFrom, TryInto},
    path::Path,
};
use surf::{
    http::headers,
    middleware::{self, Middleware},
    StatusCode,
};

struct API;

#[surf::utils::async_trait]
impl Middleware for API {
    async fn handle(
        &self,
        mut req: middleware::Request,
        client: std::sync::Arc<dyn middleware::HttpClient>,
        next: middleware::Next<'_>,
    ) -> surf::Result<middleware::Response> {
        req.insert_header(headers::USER_AGENT, "updatehub/next");
        req.insert_header(headers::CONTENT_TYPE, "application/json");
        req.insert_header("api-content-type", "application/vnd.updatehub-v1+json");
        Ok(next.run(req, client).await?)
    }
}

pub struct Client<'a> {
    client: surf::Client,
    server: &'a str,
}

pub async fn get<W>(url: &str, handle: &mut W) -> Result<()>
where
    W: io::Write + Unpin,
{
    validate_url(url)?;
    let req = surf::get(url);
    save_body_to(req, handle).await
}

async fn save_body_to<W>(req: surf::Request, handle: &mut W) -> Result<()>
where
    W: io::Write + Unpin,
{
    use io::prelude::{ReadExt, WriteExt};
    use std::str::FromStr;

    let mut rep = req.await?;
    if !rep.status().is_success() {
        return Err(Error::InvalidStatusResponse(rep.status()));
    }

    let mut written: f32 = 0.;
    let mut threshold = 10;
    let length = match rep.header(headers::CONTENT_LENGTH) {
        Some(v) => usize::from_str(v.as_str())?,
        None => 0,
    };

    loop {
        let mut buf = [0; 4096];
        let read = rep.read(&mut buf).await?;
        if read == 0 {
            break;
        }
        handle.write_all(&buf[..read]).await?;
        if length > 0 {
            written += read as f32 / (length as f32 / 100.);
            if written as usize >= threshold {
                threshold += 20;
                debug!("{}% of the file has been downloaded", std::cmp::min(written as usize, 100));
            }
        }
    }
    debug!("100% of the file has been downloaded");

    Ok(())
}

impl<'a> Client<'a> {
    pub fn new(server: &'a str) -> Self {
        Self { server, client: surf::Client::new() }
    }

    pub async fn probe(
        &self,
        num_retries: u64,
        firmware: api::FirmwareMetadata<'_>,
    ) -> Result<api::ProbeResponse> {
        validate_url(self.server)?;

        let mut response = self
            .client
            .post(&format!("{}/upgrades", &self.server))
            .middleware(API)
            .set_header("api-retries", num_retries.to_string())
            .body_json(&firmware)?
            .await?;

        match response.status() {
            StatusCode::NotFound => Ok(api::ProbeResponse::NoUpdate),
            StatusCode::Ok => {
                match response
                    .header("add-extra-poll")
                    .map(|extra_poll| extra_poll.as_str())
                    .and_then(|extra_poll| extra_poll.parse().ok())
                {
                    Some(extra_poll) => Ok(api::ProbeResponse::ExtraPoll(extra_poll)),
                    None => {
                        let signature =
                            response.header("UH-Signature").map(TryInto::try_into).transpose()?;
                        Ok(api::ProbeResponse::Update(
                            api::UpdatePackage::parse(&response.body_bytes().await?)?,
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
        validate_url(self.server)?;

        // FIXME: Discuss the need of packages inside the route
        let mut request = self
            .client
            .get(&format!(
                "{}/products/{}/packages/{}/objects/{}",
                &self.server, product_uid, package_uid, object
            ))
            .middleware(API);

        if !download_dir.exists() {
            fs::create_dir_all(download_dir).await.map_err(|e| {
                error!("fail to create {:?} directory, error: {}", download_dir, e);
                e
            })?;
        }

        let file = download_dir.join(object);
        if file.exists() {
            request = request.set_header(
                "RANGE",
                format!("bytes={}-", file.metadata()?.len().saturating_sub(1)),
            );
        }

        let mut file = fs::OpenOptions::new().create(true).append(true).open(&file).await?;

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
        validate_url(self.server)?;

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

        let payload =
            Payload { state, firmware, package_uid, previous_state, error_message, current_log };

        self.client
            .post(&format!("{}/report", &self.server))
            .middleware(API)
            .body_json(&payload)?
            .await?;
        Ok(())
    }
}

impl TryFrom<&headers::HeaderValues> for api::Signature {
    type Error = Error;

    fn try_from(value: &headers::HeaderValues) -> Result<Self> {
        let value = value.as_str();

        // Workarround for https://github.com/sfackler/rust-openssl/issues/1325
        if value.is_empty() {
            return Ok(Self::from_base64_str("")?);
        }

        Ok(Self::from_base64_str(value)?)
    }
}

fn validate_url(url: &str) -> surf::Result<()> {
    surf::http::Url::parse(url)?;
    Ok(())
}
