// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::{firmware::Metadata, runtime_settings::RuntimeSettings, update_package::UpdatePackage};

use failure::bail;
use reqwest::{
    header::{HeaderMap, HeaderName, CONTENT_TYPE, RANGE, USER_AGENT},
    Client, StatusCode,
};
use serde::Serialize;
use slog_scope::debug;
use std::{path::Path, time::Duration};

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

impl<'a> Api<'a> {
    pub(crate) fn new(server: &'a str) -> Self {
        Self { server }
    }

    fn client(&self) -> Result<Client, failure::Error> {
        let mut headers = HeaderMap::new();

        headers.insert(USER_AGENT, "updatehub/next".parse()?);
        headers.insert(CONTENT_TYPE, "application/json".parse()?);
        headers.insert(
            HeaderName::from_static("api-content-type"),
            "application/vnd.updatehub-v1+json".parse()?,
        );

        Ok(Client::builder().timeout(Duration::from_secs(10)).default_headers(headers).build()?)
    }

    pub fn probe(
        &self,
        runtime_settings: &RuntimeSettings,
        firmware: &Metadata,
    ) -> Result<ProbeResponse, failure::Error> {
        let mut response = self
            .client()?
            .post(&format!("{}/upgrades", &self.server))
            .header(HeaderName::from_static("api-retries"), runtime_settings.retries())
            .json(firmware)
            .send()?;

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
                    None => Ok(ProbeResponse::Update(UpdatePackage::parse(&response.text()?)?)),
                }
            }
            _ => bail!("Invalid response. Status: {}", response.status()),
        }
    }

    pub fn download_object(
        &self,
        product_uid: &str,
        package_uid: &str,
        download_dir: &Path,
        object: &str,
    ) -> Result<(), failure::Error> {
        use std::fs::{create_dir_all, OpenOptions};

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

        let mut file = OpenOptions::new().create(true).append(true).open(&file)?;
        let mut response = client.send()?;
        if response.status().is_success() {
            response.copy_to(&mut file)?;
            return Ok(());
        }

        bail!("Couldn't download the object {}", object)
    }

    pub fn report(
        &self,
        state: &str,
        firmware: &Metadata,
        package_uid: &str,
        previous_state: Option<&str>,
        error_message: Option<String>,
        current_log: Option<String>,
    ) -> Result<(), failure::Error> {
        #[derive(Serialize)]
        #[serde(rename_all = "kebab-case")]
        struct Payload<'a> {
            #[serde(rename = "status")]
            state: &'a str,
            #[serde(flatten)]
            firmware: &'a Metadata,
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

        self.client()?.post(&format!("{}/report", &self.server)).json(&payload).send()?;
        Ok(())
    }
}
