// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
//

use Result;

use reqwest::header::{ByteRangeSpec, ContentType, Headers, Range, UserAgent};
use reqwest::{Client, StatusCode};

use std::time::Duration;

use firmware::Metadata;
use runtime_settings::RuntimeSettings;
use settings::Settings;

use update_package::UpdatePackage;

#[cfg(test)]
pub mod tests;

header! { (ApiContentType, "Api-Content-Type") => [String] }
header! { (ApiRetries, "Api-Retries") => [usize] }
header! { (AddExtraPoll, "Add-Extra-Poll") => [i64] }

pub struct Api<'a> {
    settings: &'a Settings,
    firmware: &'a Metadata,
    runtime_settings: &'a RuntimeSettings,
}

#[derive(Debug)]
pub enum ProbeResponse {
    NoUpdate,
    Update(UpdatePackage),
    ExtraPoll(i64),
}

impl<'a> Api<'a> {
    pub fn new(
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
        let mut headers = Headers::new();

        headers.set(UserAgent::new("updatehub/next"));
        headers.set(ContentType::json());
        headers.set(ApiContentType("application/vnd.updatehub-v1+json".into()));

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
            )).header(ApiRetries(self.runtime_settings.polling.retries))
            .json(&self.firmware)
            .send()?;

        match response.status() {
            StatusCode::NotFound => Ok(ProbeResponse::NoUpdate),
            StatusCode::Ok => {
                if let Some(extra_poll) = response.headers().get::<AddExtraPoll>() {
                    return Ok(ProbeResponse::ExtraPoll(extra_poll.0));
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
            client.header(Range::Bytes(vec![ByteRangeSpec::AllFrom(
                file.metadata()?.len() - 1,
            )]));
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
