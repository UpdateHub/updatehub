// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use mockito::{mock, Mock};
use regex::Regex;
use serde_json::json;
use std::{env, path::PathBuf};

pub enum FakeServer {
    NoUpdate,
    HasUpdate(String),
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
}

impl Default for Settings {
    fn default() -> Self {
        Settings {
            polling: false,
            listen_socket: String::from("localhost:8080"),
            server_address: mockito::server_url(),
            download_dir: None,
            config_file: None,
            timeout: None,
        }
    }
}

impl Settings {
    pub fn init_server(self) -> (rexpect::session::PtySession, updatehub::tests::TestEnvironment) {
        let setup = updatehub::tests::TestEnvironment::build()
            .listen_socket(self.listen_socket)
            .server_address(self.server_address)
            .add_echo_binary("reboot");

        let mut setup = if !self.polling { setup.disable_polling() } else { setup }.finish();

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
            "{} server -v trace -c {}",
            cargo_bin("updatehub").to_string_lossy(),
            setup.settings.stored_path.to_string_lossy()
        );

        let handle = rexpect::spawn(&cmd, self.timeout).expect("fail to spawn server command");

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
}

pub fn get_output_server(handle: &mut rexpect::session::PtySession, polling: Polling) -> String {
    handle
        .exp_regex(match polling {
            Polling::Enable => "\r\n.* TRCE delaying transition for:.*",
            Polling::Disable => "\r\n.* TRCE stopping transition until awoken.*",
        })
        .expect("fail to match the required string")
        .0
}

pub fn run_client_probe(server: Server) -> String {
    let cmd_string = format!("{} client probe", cargo_bin("updatehub").to_string_lossy());
    let cmd = match server {
        Server::Custom(server_address) => format!("{} --server {}", cmd_string, server_address),
        Server::Standard => cmd_string,
    };
    let mut handle = rexpect::spawn(&cmd, None).expect("fail to spawn probe command");
    handle.exp_eof().expect("fail to match the EOF for client")
}

pub fn run_client_log() -> String {
    let cmd = format!("{} client log", cargo_bin("updatehub").to_string_lossy());
    let mut handle = rexpect::spawn(&cmd, None).expect("fail to spawn log command");
    handle.exp_eof().expect("fail to match the EOF for client")
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
        ]
    }
}

pub fn format_output_server(s: String) -> (String, String) {
    let version_re = Regex::new(r"Agent .*").unwrap();
    let tmpfile_re = Regex::new(r#""/tmp/.tmp.*""#).unwrap();
    let date_re = Regex::new(r"\b(?:Jan|...|Dec) (\d{2}) (\d{2}):(\d{2}):(\d{2}).(\d{3})").unwrap();
    let trce_re = Regex::new(r"<timestamp> TRCE.*").unwrap();
    let debg_re = Regex::new(r"<timestamp> DEBG.*").unwrap();
    let download_re = Regex::new(r"DEBG (\d{2})%").unwrap();

    let s = version_re.replace_all(&s, "Agent <version>");
    let s = tmpfile_re.replace_all(&s, r#""<file>""#);
    let s = date_re.replace_all(&s, "<timestamp>");
    let s = download_re.replace_all(&s, "DEBG <percentage>%");
    let s_trce = s.replace("\r\n", "\n");

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

pub fn remove_carriage_newline_caracters(s: String) -> String {
    s.replace("\r\n", "\n")
}

pub fn format_output_client_log(s: String) -> String {
    let date_re =
        Regex::new(r"(\d{4})-(\d{2})-(\d{2}) (\d{2}):(\d{2}):(\d{2}).(\d{9}) (-|\+)(\d{4})")
            .expect("fail to compile the date regexp");
    let s = date_re.replace_all(&s, "<timestamp>");

    let tmpfile_re = Regex::new(r#"\\"/tmp/.tmp.*""#).expect("fail to compile the tmpfile regexp");
    let s = tmpfile_re.replace_all(&s, r#""<file>""#);

    remove_carriage_newline_caracters(s.to_string())
}
