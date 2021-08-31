// Copyright (C) 2021 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

logging_content::trait_LogContent!(slog_scope);
logging_content::trait_LogDisplay!();
logging_content::impl_Result_no_ok!();

impl<E: std::error::Error> LogDisplay for E {
    fn as_log_display(&self, _: logging_content::Level) -> String {
        format!("{} ({:?})", self, self)
    }
}
