// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

pub mod info;
pub mod probe {
    use serde::{Deserialize, Serialize};

    #[derive(Clone, Debug, Deserialize, Serialize)]
    #[serde(deny_unknown_fields)]
    pub struct Request {
        pub custom_server: String,
    }

    #[derive(Clone, Debug, Deserialize, Serialize)]
    #[serde(rename_all = "snake_case")]
    pub enum Response {
        Updating,
        NoUpdate,
        TryAgain(i64),
    }
}

pub mod local_install {
    use serde::{Deserialize, Serialize};

    #[derive(Deserialize, Clone, Debug, Serialize)]
    #[serde(deny_unknown_fields)]
    pub struct Request {
        pub file: std::path::PathBuf,
    }
}

pub mod remote_install {
    use serde::{Deserialize, Serialize};

    #[derive(Deserialize, Clone, Debug, Serialize)]
    #[serde(deny_unknown_fields)]
    pub struct Request {
        pub url: String,
    }
}

pub mod state {
    use serde::{Deserialize, Serialize};

    #[derive(Clone, Debug, Deserialize, Serialize)]
    #[serde(rename_all = "lowercase")]
    pub enum Response {
        Park,
        EntryPoint,
        Poll,
        Probe,
        Validation,
        Download,
        Install,
        Reboot,
        DirectDownload,
        PrepareLocalInstall,
        Error,
    }
}

pub mod abort_download {
    use serde::{Deserialize, Serialize};

    #[derive(Clone, Debug, Deserialize, Serialize)]
    #[serde(deny_unknown_fields)]
    pub struct Response {
        pub message: String,
    }

    #[derive(Clone, Debug, Deserialize, Serialize)]
    #[serde(deny_unknown_fields)]
    pub struct Refused {
        pub error: String,
    }
}

pub mod log {
    use serde::{Deserialize, Serialize};
    use std::collections::HashMap;

    #[derive(Clone, Debug, Deserialize, Serialize)]
    #[serde(rename_all = "lowercase")]
    pub enum Level {
        Critical,
        Error,
        Warning,
        Info,
        Debug,
        Trace,
    }

    #[derive(Clone, Debug, Deserialize, Serialize)]
    #[serde(deny_unknown_fields)]
    pub struct Log {
        entries: Vec<Entry>,
    }

    #[derive(Clone, Debug, Deserialize, Serialize)]
    #[serde(deny_unknown_fields)]
    pub struct Entry {
        level: Level,
        message: String,
        time: String,
        data: HashMap<String, String>,
    }

    impl core::fmt::Display for Log {
        fn fmt(&self, f: &mut core::fmt::Formatter) -> Result<(), core::fmt::Error> {
            for entry in &self.entries {
                let level = match entry.level {
                    Level::Critical => "CRIT",
                    Level::Error => "ERRO",
                    Level::Warning => "WARN",
                    Level::Info => "INFO",
                    Level::Debug => "DEBG",
                    Level::Trace => "TRCE",
                };

                writeln!(
                    f,
                    "{timestamp} {level} {msg}",
                    timestamp = entry.time,
                    level = level,
                    msg = entry.message
                )?;
            }
            Ok(())
        }
    }
}
