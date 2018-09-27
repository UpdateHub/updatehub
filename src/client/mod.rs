// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use Result;

use reqwest::{
    header::{HeaderMap, HeaderName, CONTENT_TYPE, RANGE, USER_AGENT},
    Client, StatusCode,
};

use std::time::Duration;

use firmware::Metadata;
use runtime_settings::RuntimeSettings;
use settings::Settings;

use update_package::UpdatePackage;

#[cfg(test)]
pub(crate) mod tests;

pub(crate) struct Api<'a> {
    settings: &'a Settings,
    firmware: &'a Metadata,
    runtime_settings: &'a RuntimeSettings,
}

#[derive(Debug)]
pub(crate) enum ProbeResponse {
    NoUpdate,
    Update(UpdatePackage),
    ExtraPoll(i64),
}

impl<'a> Api<'a> {
    pub(crate) fn new(
        settings: &'a Settings,
        runtime_settings: &'a RuntimeSettings,
        firmware: &'a Metadata,
    ) -> Api<'a> {
        Api {
            settings,
            runtime_settings,
            firmware,
        }
    }

    fn client(&self) -> Result<Client> {
        let mut headers = HeaderMap::new();

        headers.insert(USER_AGENT, "updatehub/next".parse()?);
        headers.insert(CONTENT_TYPE, "application/json".parse()?);
        headers.insert(
            HeaderName::from_static("api-content-type"),
            "application/vnd.updatehub-v1+json".parse()?,
        );

        Ok(Client::builder()
            .timeout(Duration::from_secs(10))
            .default_headers(headers)
            .build()?)
    }

    pub fn probe(&self) -> Result<ProbeResponse> {
        let mut response = self
            .client()?
            .post(&format!(
                "{}/upgrades",
                &self.settings.network.server_address
            ))
            .header(
                HeaderName::from_static("api-retries"),
                self.runtime_settings.retries(),
            )
            .json(&self.firmware)
            .send()?;

        match response.status() {
            StatusCode::NOT_FOUND => Ok(ProbeResponse::NoUpdate),
            StatusCode::OK => {
                if let Some(extra_poll) = response
                    .headers()
                    .get("add-extra-poll")
                    .and_then(|extra_poll| extra_poll.to_str().ok())
                    .and_then(|extra_poll| extra_poll.parse().ok())
                {
                    return Ok(ProbeResponse::ExtraPoll(extra_poll));
                }

                Ok(ProbeResponse::Update(UpdatePackage::parse(
                    &response.text()?,
                )?))
            }
            _ => bail!("Invalid response. Status: {}", response.status()),
        }
    }

    pub fn download_object(&self, package_uid: &str, object: &str) -> Result<()> {
        use std::fs::{create_dir_all, OpenOptions};

        // FIXME: Discuss the need of packages inside the route
        let mut client = self.client()?.get(&format!(
            "{}/products/{}/packages/{}/objects/{}",
            &self.settings.network.server_address, &self.firmware.product_uid, package_uid, object
        ));

        let path = &self.settings.update.download_dir;
        if !&path.exists() {
            debug!("Creating directory to store the downloads.");
            create_dir_all(&path)?;
        }

        let file = path.join(object);
        if file.exists() {
            client = client.header(RANGE, format!("bytes={}-", file.metadata()?.len() - 1));
        }

        let mut file = OpenOptions::new().create(true).append(true).open(&file)?;
        let mut response = client.send()?;
        if response.status().is_success() {
            response.copy_to(&mut file)?;
            return Ok(());
        }

        bail!("Couldn't download the object {}", object)
    }
}
