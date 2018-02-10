// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
// 

use failure::Error;
use reqwest::Client;
use reqwest::header::{ContentType, Headers, UserAgent};

use std::time::Duration;

use firmware::Metadata;
use runtime_settings::RuntimeSettings;
use settings::Settings;

use update_package::UpdatePackage;

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

    fn client(&self) -> Result<Client, Error> {
        let mut headers = Headers::new();

        headers.set(UserAgent::new("updatehub/next"));
        headers.set(ContentType::json());
        headers.set(ApiContentType("application/vnd.updatehub-v1+json".into()));

        Ok(Client::builder()
            .timeout(Duration::from_secs(10))
            .default_headers(headers)
            .build()?)
    }

    pub fn probe(&self) -> Result<ProbeResponse, Error> {
        use reqwest::StatusCode;

        let mut response = self.client()?
            .post(&format!(
                "{}/upgrades",
                &self.settings.network.server_address
            ))
            .header(ApiRetries(self.runtime_settings.polling.retries))
            .json(&self.firmware)
            .send()?;

        match response.status() {
            StatusCode::NotFound => Ok(ProbeResponse::NoUpdate),
            StatusCode::Ok => {
                if let Some(extra_poll) = response.headers().get::<AddExtraPoll>() {
                    return Ok(ProbeResponse::ExtraPoll(extra_poll.0));
                }

                Ok(ProbeResponse::Update(
                    UpdatePackage::parse(&response.text()?)?,
                ))
            }
            _ => bail!("Invalid response. Status: {}", response.status()),
        }
    }

    pub fn download_object(&self, package_uid: &str, object: &str) -> Result<(), Error> {
        // FIXME: Discuss the need of packages inside the route
        let mut client = self.client()?.get(&format!(
            "{}/products/{}/packages/{}/objects/{}",
            &self.settings.network.server_address, &self.firmware.product_uid, package_uid, object
        ));

        let _ = client.send()?;

        Ok(())
    }
}

#[cfg(test)]
pub mod tests {
    use super::*;
    use mockito::Mock;

    pub enum FakeServer {
        NoUpdate,
        HasUpdate,
        ExtraPoll,
        ErrorOnce,
        InvalidHardware,
    }

    pub fn create_mock_server(server: FakeServer) -> Mock {
        use mockito::{mock, Matcher};
        use update_package::tests::get_update_json;

        match server {
            FakeServer::NoUpdate => mock("POST", "/upgrades")
                .match_header("Content-Type", "application/json")
                .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
                .match_body(Matcher::JSON(json!(
                        {
                            "product_uid": "229ffd7e08721d716163fc81a2dbaf6c90d449f0a3b009b6a2defe8a0b0d7381",
                            "version": "1.1",
                            "hardware": "board",
                            "device_identity": {
                                "id1":["value1"],
                                "id2":["value2"]
                            },
                            "device_attributes": {
                                "attr1":["attrvalue1"],
                                "attr2":["attrvalue2"]
                            }
                        }
                    )))
                .with_status(404)
                .create(),

            FakeServer::HasUpdate => mock("POST", "/upgrades")
                .match_header("Content-Type", "application/json")
                .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
                .match_body(Matcher::JSON(json!(
                        {
                            "product_uid": "229ffd7e08721d716163fc81a2dbaf6c90d449f0a3b009b6a2defe8a0b0d7381",
                            "version": "1.1",
                            "hardware": "board",
                            "device_identity": {
                                "id1":["value2"],
                                "id2":["value2"]
                            },
                            "device_attributes": {
                                "attr1":["attrvalue1"],
                                "attr2":["attrvalue2"]
                            }
                        }
                    )))
                .with_status(200)
                .with_body(&get_update_json().to_string())
                .create(),

            FakeServer::ExtraPoll => mock("POST", "/upgrades")
                .match_header("Content-Type", "application/json")
                .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
                .match_body(Matcher::JSON(json!(
                                             {
                                                 "product_uid": "229ffd7e08721d716163fc81a2dbaf6c90d449f0a3b009b6a2defe8a0b0d7381",
                                                 "version": "1.1",
                                                 "hardware": "board",
                                                 "device_identity": {
                                                     "id1":["value3"],
                                                     "id2":["value2"]
                                                 },
                                                 "device_attributes": {
                                                     "attr1":["attrvalue1"],
                                                     "attr2":["attrvalue2"]
                                                 }
                                             }
                                         )))
                .with_status(200)
                .with_header("Add-Extra-Poll", "10")
                .create(),

            FakeServer::ErrorOnce => mock("POST", "/upgrades")
                .match_header("Content-Type", "application/json")
                .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
                .match_header("Api-Retries", "1")
                .match_body(Matcher::JSON(json!(
                                             {
                                                 "product_uid": "229ffd7e08721d716163fc81a2dbaf6c90d449f0a3b009b6a2defe8a0b0d7381",
                                                 "version": "1.1",
                                                 "hardware": "board",
                                                 "device_identity": {
                                                     "id1":["value1"],
                                                     "id2":["value2"]
                                                 },
                                                 "device_attributes": {
                                                     "attr1":["attrvalue1"],
                                                     "attr2":["attrvalue2"]
                                                 },
                                             }
                                         )))
                .with_status(404)
                .create(),
            FakeServer::InvalidHardware => mock("POST", "/upgrades")
                .match_header("Content-Type", "application/json")
                .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
                .match_body(Matcher::JSON(json!(
                                             {
                                                 "product_uid": "229ffd7e08721d716163fc81a2dbaf6c90d449f0a3b009b6a2defe8a0b0d7381",
                                                 "version": "1.1",
                                                 "hardware": "invalid",
                                                 "device_identity": {
                                                     "id1":["value4"],
                                                     "id2":["value2"]
                                                 },
                                                 "device_attributes": {
                                                     "attr1":["attrvalue1"],
                                                     "attr2":["attrvalue2"]
                                                 }
                                             }
                                         )))
                .with_status(200)
                .with_body(&get_update_json().to_string())
                .create(),
        }
    }

    #[test]
    fn probe_requirements() {
        use firmware::tests::{create_fake_metadata, FakeDevice};

        let mock = create_mock_server(FakeServer::NoUpdate);
        let _ = Api::new(
            &Settings::default(),
            &RuntimeSettings::default(),
            &Metadata::new(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap(),
        ).probe();
        mock.assert();
    }

    #[test]
    fn download_object() {
        use firmware::tests::{create_fake_metadata, FakeDevice};
        use mockito::mock;

        let metadata = Metadata::new(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap();

        let m = mock(
            "GET",
            format!(
                "/products/{}/packages/{}/objects/{}",
                metadata.product_uid, "package_id", "object"
            ).as_str(),
        ).match_header("Content-Type", "application/json")
            .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
            .with_status(200)
            .create();

        let _ = Api::new(&Settings::default(), &RuntimeSettings::default(), &metadata)
            .download_object("package_id", "object")
            .unwrap();

        m.assert();
    }
}
