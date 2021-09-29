// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use mockito::{mock, Mock};
use regex::Regex;
use serde_json::json;
use std::{env, net::TcpListener, path::PathBuf};

pub enum FakeServer {
    NoUpdate,
    HasUpdate(String),
    CheckRequirementsTest(String),
    RemoteInstall,
}

pub enum StopMessage {
    Custom(String),
    Polling(Polling),
}

pub enum Polling {
    Enable,
    Disable,
}

pub enum Server {
    Custom(String),
    Standard,
}

pub struct Settings {
    polling: bool,
    listen_socket: String,
    server_address: String,
    download_dir: Option<PathBuf>,
    config_file: Option<PathBuf>,
    timeout: Option<u64>,
    install_modes: Option<Vec<&'static str>>,
    state_change_callback: Option<&'static str>,
    validate_callback: Option<&'static str>,
    booting_from_update: bool,
}

impl Default for Settings {
    fn default() -> Self {
        Settings {
            polling: false,
            listen_socket: format!(
                "localhost:{}",
                listen_available_port().expect("failed to bind a socket.")
            ),
            server_address: mockito::server_url(),
            download_dir: None,
            config_file: None,
            timeout: None,
            install_modes: None,
            state_change_callback: None,
            validate_callback: None,
            booting_from_update: false,
        }
    }
}

impl Settings {
    pub fn init_server(self) -> (expectrl::Session, updatehub::tests::TestEnvironment) {
        let mut setup = updatehub::tests::TestEnvironment::build()
            .listen_socket(self.listen_socket)
            .server_address(self.server_address)
            .add_echo_binary("reboot");

        if self.booting_from_update {
            setup = setup.booting_from_update();
        }
        if let Some(s) = self.state_change_callback {
            setup = setup.state_change_callback(s.to_owned());
        }
        if let Some(s) = self.validate_callback {
            setup = setup.validate_callback(s.to_owned());
        }
        if let Some(l) = self.install_modes {
            setup = setup.supported_install_modes(l)
        }
        if !self.polling {
            setup = setup.disable_polling()
        }
        let mut setup = setup.finish();

        if let Some(download_dir) = self.download_dir {
            setup.settings.data.update.download_dir = download_dir;

            let content = toml::ser::to_string_pretty(&setup.settings.data.0)
                .expect("fail to convert the data to toml");
            std::fs::write(&setup.settings.stored_path, content)
                .expect("fail to write the content on settings file");
        }

        if let Some(config_file) = self.config_file {
            setup.settings.stored_path = config_file;
        }

        let cmd = format!(
            "{} daemon -v trace -c {}",
            cargo_bin("updatehub").to_string_lossy(),
            setup.settings.stored_path.to_string_lossy()
        );

        let mut handle = expectrl::spawn(&cmd).expect("fail to spawn server command");
        handle.set_expect_timeout(self.timeout.map(std::time::Duration::from_secs));

        (handle, setup)
    }

    pub fn timeout(self, t: u64) -> Self {
        Settings { timeout: Some(t), ..self }
    }

    pub fn config_file(self, p: PathBuf) -> Self {
        Settings { config_file: Some(p), ..self }
    }

    pub fn download_dir(self, p: PathBuf) -> Self {
        Settings { download_dir: Some(p), ..self }
    }

    pub fn polling(self) -> Self {
        Settings { polling: true, ..self }
    }

    pub fn listen_socket(self, s: String) -> Self {
        Settings { listen_socket: s, ..self }
    }

    pub fn server_address(self, s: String) -> Self {
        Settings { server_address: s, ..self }
    }

    pub fn supported_install_modes(self, l: Vec<&'static str>) -> Self {
        Settings { install_modes: Some(l), ..self }
    }

    pub fn state_change_callback(self, s: &'static str) -> Self {
        Settings { state_change_callback: Some(s), ..self }
    }

    pub fn validate_callback(self, s: &'static str) -> Self {
        Settings { validate_callback: Some(s), ..self }
    }

    pub fn booting_from_update(self) -> Self {
        Settings { booting_from_update: true, ..self }
    }
}

pub fn get_output_server(
    handle: &mut expectrl::Session,
    stop_message: StopMessage,
) -> (String, String) {
    let stdout = String::from_utf8_lossy(
        handle
            .expect(expectrl::Regex(match stop_message {
                StopMessage::Custom(ref s) => s,
                StopMessage::Polling(Polling::Enable) => {
                    "\r\n.* TRCE delaying transition for: .* seconds$"
                }
                StopMessage::Polling(Polling::Disable) => {
                    "\r\n.* TRCE stopping transition until awoken$"
                }
            }))
            .expect("fail to match the required string")
            .before(),
    )
    .into_owned();

    rewrite_log_output(stdout)
}

pub fn run_client_probe(server: Server, daemon_address: &str) -> String {
    let cmd_string = format!(
        "{} client --daemon-address {} probe",
        cargo_bin("updatehub").to_string_lossy(),
        daemon_address
    );
    let cmd = match server {
        Server::Custom(server_address) => format!("{} --server {}", cmd_string, server_address),
        Server::Standard => cmd_string,
    };
    let mut handle = expectrl::spawn(&cmd).expect("fail to spawn probe command");
    let m = handle.expect(expectrl::Eof).expect("fail to match the EOF for client");
    String::from_utf8_lossy(m.first()).into_owned()
}

pub fn run_client_local_install(mock_addr: &str, daemon_address: &str) -> String {
    let cmd = format!(
        "{} client --daemon-address {} install-package {}/some-direct-package-url",
        cargo_bin("updatehub").to_string_lossy(),
        daemon_address,
        mock_addr
    );
    let mut handle = expectrl::spawn(&cmd).expect("fail to spawn probe command");
    let m = handle.expect(expectrl::Eof).expect("fail to match the EOF for client");
    String::from_utf8_lossy(m.first()).into_owned()
}

pub fn run_client_log(daemon_address: &str) -> String {
    let cmd = format!(
        "{} client --daemon-address {} log",
        cargo_bin("updatehub").to_string_lossy(),
        daemon_address
    );
    let mut handle = expectrl::spawn(&cmd).expect("fail to spawn log command");
    handle.set_expect_timeout(Some(std::time::Duration::from_secs(60)));
    let m = handle.expect(expectrl::Eof).expect("fail to match the EOF for client");
    rewrite_log_output(String::from_utf8_lossy(m.first()).into_owned()).0
}

pub fn cargo_bin<S: AsRef<str>>(name: S) -> PathBuf {
    let mut target_dir = env::current_exe().expect("fail to get current binary name");

    target_dir.pop();
    if target_dir.ends_with("deps") {
        target_dir.pop();
    }

    target_dir.join(format!("{}{}", name.as_ref(), env::consts::EXE_SUFFIX))
}

pub fn create_mock_server(server: FakeServer) -> Vec<Mock> {
    use mockito::Matcher;

    let json_update = json!({
        "product": "0123456789",
        "version": "1.2",
        "supported-hardware": ["board"],
        "objects":
        [
            [
                {
                    "mode": "test",
                    "filename": "testfile",
                    "target": "/dev/device1",
                    "sha256sum": "23c3c412177bd37b9b61bf4738b18dc1fe003811c2583a14d2d9952d8b6a75b4",
                    "size": 40960,
                    "force-check-requirements-fail": false
                }
            ],
            [
                {
                    "mode": "test",
                    "filename": "testfile",
                    "target": "/dev/device2",
                    "sha256sum": "23c3c412177bd37b9b61bf4738b18dc1fe003811c2583a14d2d9952d8b6a75b4",
                    "size": 40960,
                    "force-check-requirements-fail": false
                }
            ]
        ]
    });

    let wrong_json_update = json!({
        "product": "0123456789",
        "version": "1.2",
        "supported-hardware": ["board"],
        "objects":
        [
            [
                {
                    "mode": "test",
                    "filename": "testfile",
                    "target": "/dev/device1",
                    "sha256sum": "23c3c412177bd37b9b61bf4738b18dc1fe003811c2583a14d2d9952d8b6a75b4",
                    "size": 40960,
                    "force-check-requirements-fail": true
                }
            ],
            [
                {
                    "mode": "test",
                    "filename": "testfile",
                    "target": "/dev/device2",
                    "sha256sum": "23c3c412177bd37b9b61bf4738b18dc1fe003811c2583a14d2d9952d8b6a75b4",
                    "size": 40960,
                    "force-check-requirements-fail": true
                }
            ]
        ]
    });

    let request_body = Matcher::Json(json!({
        "product-uid": "229ffd7e08721d716163fc81a2dbaf6c90d449f0a3b009b6a2defe8a0b0d7381",
        "version": "1.1",
        "hardware": "board",
        "device-identity": {
            "id1":"value1",
            "id2":"value2"
        },
        "device-attributes": {
            "attr1":"attrvalue1",
            "attr2":"attrvalue2"
        }
    }));

    match server {
        FakeServer::NoUpdate => vec![
            mock("POST", "/upgrades")
                .match_header("Content-Type", "application/json")
                .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
                .match_body(request_body)
                .expect_at_least(1)
                .with_status(404)
                .create(),
        ],
        FakeServer::HasUpdate(product_uid) => vec![
            mock("POST", "/upgrades")
                .match_header("Content-Type", "application/json")
                .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
                .match_body(request_body)
                .with_status(200)
                .with_header("UH-Signature", &openssl::base64::encode_block(b"some_signature"))
                .with_body(&json_update.to_string())
                .create(),
            mock(
                "GET",
                format!("/products/{}/packages/87effe73b80453f397cee4db3c3589a8630b220876dff8fb23447315037ff96d/objects/23c3c412177bd37b9b61bf4738b18dc1fe003811c2583a14d2d9952d8b6a75b4", product_uid)
                    .as_str(),
            )
            .match_header("Content-Type", "application/json")
            .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
            .with_status(200)
                .with_body(std::iter::repeat(0xF).take(40960).collect::<Vec<_>>())
            .create(),
        ],
        FakeServer::CheckRequirementsTest(product_uid) => vec![
            mock("POST", "/upgrades")
                .match_header("Content-Type", "application/json")
                .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
                .match_body(request_body)
                .with_status(200)
                .with_header("UH-Signature", &openssl::base64::encode_block(b"some_signature"))
                .with_body(&wrong_json_update.to_string())
                .create(),
            mock(
                "GET",
                format!("/products/{}/packages/fb21b217cb83e8af368c773eb13bad0a94e1b0088c6bf561072decf3c1ae9df3/objects/23c3c412177bd37b9b61bf4738b18dc1fe003811c2583a14d2d9952d8b6a75b4", product_uid)
                    .as_str(),
            )
            .match_header("Content-Type", "application/json")
            .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
            .with_status(200)
                .with_body(std::iter::repeat(0xF).take(40960).collect::<Vec<_>>())
            .create(),
        ],
        FakeServer::RemoteInstall => {
            let test_uhupkg = format!("{}/fixtures/test.uhupkg", env!("CARGO_MANIFEST_DIR"));
            vec![
            mock("get", "/some-direct-package-url")
                .with_status(200)
                    .with_body_from_file(test_uhupkg)
                .create(),
        ]},
    }
}

pub fn rewrite_log_output(s: String) -> (String, String) {
    let version_re = Regex::new(r"Agent .*").unwrap();
    let tmpfile_re = Regex::new(r#""/.*/.tmp.*""#).unwrap();
    let date_re = Regex::new(r"\b(?:Jan|...|Dec) (\d{2}) (\d{2}):(\d{2}):(\d{2}).(\d{3})").unwrap();
    let time_re = Regex::new(r#"(\d{5}) seconds"#).unwrap();
    let trce_re = Regex::new(r"<timestamp> TRCE.*").unwrap();
    let debg_re = Regex::new(r"<timestamp> DEBG.*").unwrap();
    let download_re = Regex::new(r"DEBG (\d{2})%").unwrap();

    let s = version_re.replace_all(&s, "Agent <version>");
    let s = tmpfile_re.replace_all(&s, r#""<file>""#);
    let s = date_re.replace_all(&s, "<timestamp>");
    let s = download_re.replace_all(&s, "DEBG <percentage>%");
    let s = time_re.replace_all(&s, r#"<time>"#);
    let s_trce = s.replace("\r\n", "\n").trim().to_owned();

    let s_info = trce_re.replace_all(&s_trce, "");
    let s_info = debg_re.replace_all(&s_info, "");
    let s_info = s_info
        .split('\n')
        .map(|s| s.trim())
        .filter(|s| !s.is_empty())
        .collect::<Vec<_>>()
        .join("\n");

    (s_trce, s_info)
}

pub fn remove_carriage_newline_characters(s: String) -> String {
    s.replace("\r\n", "\n")
}

fn listen_available_port() -> Option<u16> {
    (8080..65535).find(|port| TcpListener::bind(("localhost", *port)).is_ok())
}
