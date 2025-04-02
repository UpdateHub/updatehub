// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

//! Contains all the structures of the request and response
//! from the agent.

/// Body of `info` response.
pub mod info;

/// Body of `probe` request and response.
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

/// Body of `local_install` request.
pub mod local_install {
    use serde::{Deserialize, Serialize};

    #[derive(Deserialize, Clone, Debug, Serialize)]
    #[serde(deny_unknown_fields)]
    pub struct Request {
        pub file: std::path::PathBuf,
    }
}

/// Body of `remote_install` request.
pub mod remote_install {
    use serde::{Deserialize, Serialize};

    #[derive(Deserialize, Clone, Debug, Serialize)]
    #[serde(deny_unknown_fields)]
    pub struct Request {
        pub url: String,
    }
}

/// Body of `state` response.
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

/// Body of `abort_download` response.
///
/// # Successful case
///
/// On a successful request, the body of response is a struct
/// called `Response` with a successful message.
///
/// # Failed case
///
/// On a failed request, the body of response is a struct
/// called `Refused` with a error message.
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

/// Body of `log` response.
pub mod log {
    use serde::{Deserialize, Serialize};
    use std::collections::HashMap;

    #[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
    #[serde(rename_all = "lowercase")]
    pub enum Level {
        // We use alias here since some string conversions of slog::Level use
        // short names even when requesting the full name:
        // https://github.com/slog-rs/slog/issues/282
        #[serde(alias = "crit")]
        Critical,
        #[serde(alias = "erro")]
        Error,
        #[serde(alias = "warn")]
        Warning,
        #[serde(alias = "info")]
        Info,
        #[serde(alias = "debg")]
        Debug,
        #[serde(alias = "trce")]
        Trace,
    }

    #[derive(Clone, Debug, Deserialize, Serialize)]
    #[serde(deny_unknown_fields)]
    pub struct Log {
        pub entries: Vec<Entry>,
    }

    #[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
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
                writeln!(f, "{entry}")?;
            }
            Ok(())
        }
    }

    impl core::fmt::Display for Entry {
        fn fmt(&self, f: &mut core::fmt::Formatter) -> Result<(), core::fmt::Error> {
            let level = match self.level {
                Level::Critical => "CRIT",
                Level::Error => "ERRO",
                Level::Warning => "WARN",
                Level::Info => "INFO",
                Level::Debug => "DEBG",
                Level::Trace => "TRCE",
            };

            write!(
                f,
                "{timestamp} {level} {msg}",
                timestamp = self.time,
                level = level,
                msg = self.message
            )?;
            Ok(())
        }
    }
}
