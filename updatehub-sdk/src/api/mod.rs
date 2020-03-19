// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

pub mod info;

pub mod probe {
    use serde::{Deserialize, Serialize};

    #[derive(Clone, Debug, Deserialize, Serialize)]
    pub struct Request {
        pub custom_server: String,
    }

    #[derive(Clone, Debug, Deserialize, Serialize)]
    pub struct Response {
        pub update_available: bool,
        #[serde(skip_serializing_if = "Option::is_none")]
        pub try_again_in: Option<i64>,
    }
}

pub mod state {
    use serde::{Deserialize, Serialize};

    #[derive(Clone, Debug, Deserialize, Serialize)]
    pub struct Response {
        pub busy: bool,
        pub current_state: String,
    }
}

pub mod abort_download {
    use serde::{Deserialize, Serialize};

    #[derive(Clone, Debug, Deserialize, Serialize)]
    pub struct Response {
        pub message: String,
    }

    #[derive(Clone, Debug, Deserialize, Serialize)]
    pub struct Refused {
        pub error: String,
    }
}

pub mod log {
    use serde::{Deserialize, Serialize};
    use std::collections::HashMap;

    #[derive(Clone, Debug, Deserialize, Serialize)]
    pub struct Entry {
        pub level: Level,
        pub message: String,
        pub time: String,
        pub data: HashMap<String, String>,
    }

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
}
