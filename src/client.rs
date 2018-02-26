// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
// 

use failure::Error;
use reqwest::{Client, StatusCode};
use reqwest::header::{ByteRangeSpec, ContentType, Headers, Range, UserAgent};

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
            client.header(Range::Bytes(vec![
                ByteRangeSpec::AllFrom(file.metadata()?.len() - 1),
            ]));
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

#[cfg(test)]
pub mod tests {
    use super::*;
    use firmware::tests::{create_fake_metadata, FakeDevice};
    use mockito::{mock, Mock};

    pub enum FakeServer {
        NoUpdate,
        HasUpdate,
        ExtraPoll,
        ErrorOnce,
        InvalidHardware,
    }

    pub fn create_mock_server(server: FakeServer) -> Mock {
        use mockito::Matcher;
        use update_package::tests::get_update_json;

        fn fake_device_reply_body(identity: usize, hardware: &str) -> Matcher {
            Matcher::JSON(json!(
                        {
                            "product_uid": "229ffd7e08721d716163fc81a2dbaf6c90d449f0a3b009b6a2defe8a0b0d7381",
                            "version": "1.1",
                            "hardware": hardware,
                            "device_identity": {
                                "id1":[format!("value{}", identity)],
                                "id2":["value2"]
                            },
                            "device_attributes": {
                                "attr1":["attrvalue1"],
                                "attr2":["attrvalue2"]
                            }
                        }
            ))
        }

        match server {
            FakeServer::NoUpdate => mock("POST", "/upgrades")
                .match_header("Content-Type", "application/json")
                .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
                .match_body(fake_device_reply_body(1, "board"))
                .with_status(404)
                .create(),

            FakeServer::HasUpdate => mock("POST", "/upgrades")
                .match_header("Content-Type", "application/json")
                .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
                .match_body(fake_device_reply_body(2, "board"))
                .with_status(200)
                .with_body(&get_update_json().to_string())
                .create(),

            FakeServer::ExtraPoll => mock("POST", "/upgrades")
                .match_header("Content-Type", "application/json")
                .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
                .match_body(fake_device_reply_body(3, "board"))
                .with_status(200)
                .with_header("Add-Extra-Poll", "10")
                .create(),

            FakeServer::ErrorOnce => mock("POST", "/upgrades")
                .match_header("Content-Type", "application/json")
                .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
                .match_header("Api-Retries", "1")
                .match_body(fake_device_reply_body(1, "board"))
                .with_status(404)
                .create(),
            FakeServer::InvalidHardware => mock("POST", "/upgrades")
                .match_header("Content-Type", "application/json")
                .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
                .match_body(fake_device_reply_body(4, "invalid"))
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
        let metadata = Metadata::new(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap();
        use mktemp::Temp;
        use std::fs::File;
        use std::io::Read;

        let m1 = mock(
            "GET",
            format!(
                "/products/{}/packages/{}/objects/{}",
                metadata.product_uid, "package_id", "object"
            ).as_str(),
        ).match_header("Content-Type", "application/json")
            .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
            .with_status(200)
            .with_body("1234")
            .create();

        let m2 = mock(
            "GET",
            format!(
                "/products/{}/packages/{}/objects/{}",
                metadata.product_uid, "package_id", "object"
            ).as_str(),
        ).match_header("Content-Type", "application/json")
            .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
            .match_header("Range", "bytes=3-")
            .with_status(200)
            .with_body("567890")
            .create();

        let mut settings = Settings::default();
        settings.update.download_dir = Temp::new_dir().unwrap().to_path_buf();

        // Download the object.
        let _ = Api::new(&settings, &RuntimeSettings::default(), &metadata)
            .download_object("package_id", "object")
            .expect("Failed to download the object.");

        // Verify it has been downloaded successfully.
        let mut downloaded = String::new();
        let _ = File::open(&settings.update.download_dir.join("object"))
            .expect("Failed to open destination object.")
            .read_to_string(&mut downloaded);

        m1.assert();

        assert_eq!(downloaded, "1234".to_string());

        // Download the remaining bytes of the object.
        let _ = Api::new(&settings, &RuntimeSettings::default(), &metadata)
            .download_object("package_id", "object")
            .expect("Failed to download the object.");

        // Verify it has been downloaded successfully.
        let mut downloaded = String::new();
        let _ = File::open(&settings.update.download_dir.join("object"))
            .expect("Failed to open destination object.")
            .read_to_string(&mut downloaded);

        m2.assert();

        assert_eq!(downloaded, "1234567890".to_string());
    }
}
