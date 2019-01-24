// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::mem_drain::MemDrain;
use lazy_static::lazy_static;
use slog::{o, Drain, Logger};
use std::{
    boxed::Box,
    sync::{Arc, Mutex},
};

lazy_static! {
    static ref BUFFER: Arc<Mutex<MemDrain>> = Arc::new(Mutex::new(MemDrain::new()));
}

pub fn init(verbose: usize) {
    let level = match verbose {
        0 => slog::Level::Info,
        1 => slog::Level::Debug,
        _ => slog::Level::Trace,
    };

    let buffer_drain = BUFFER.clone().filter_level(level).fuse();
    let terminal_drain = Mutex::new(slog_term::term_full().filter_level(level)).fuse();
    let terminal_drain = slog_async::Async::new(terminal_drain).build().fuse();

    let log = Logger::root(
        slog::Duplicate::new(buffer_drain, terminal_drain).fuse(),
        o!(),
    );
    let guard = slog_scope::set_global_logger(log);
    Box::leak(Box::new(guard));
}

pub fn buffer() -> Arc<Mutex<MemDrain>> {
    BUFFER.clone()
}

#[cfg(test)]
mod tests {
    use super::*;
    use slog::{slog_info, slog_warn};
    use slog_scope::{info, warn};

    #[test]
    fn logger_duplicated() {
        let s1 = "When logging messages after init";
        let s2 = "they should be accessible from buffer()";
        init(0);
        let buffer = buffer();
        buffer.lock().unwrap().start_logging();
        info!("{}", s1);
        info!("{}", s2);
        let result = buffer.lock().unwrap().to_string();
        assert!(result.contains(s1));
        assert!(result.contains(s2));
    }

    #[test]
    fn logger_disabled() {
        let s1 = "When the buffer is disable";
        let s2 = "No message should be stored";
        init(0);
        let buffer = buffer();
        buffer.lock().unwrap().stop_logging();
        info!("{}", s1);
        warn!("{}", s2);
        let result = buffer.lock().unwrap().to_string();
        assert!(!result.contains(s1));
        assert!(!result.contains(s2));
    }

    #[test]
    fn logger_buffer_clear() {
        let s1 = "After the buffer clear, no message should be found";
        init(0);
        let buffer = buffer();
        buffer.lock().unwrap().start_logging();
        info!("{}", s1);
        let result = buffer.lock().unwrap().to_string();
        assert!(result.contains(s1));
        buffer.lock().unwrap().clear();
        let result = buffer.lock().unwrap().to_string();
        assert!(!result.contains(s1));
    }

    #[test]
    fn logger_key_values() {
        let txt = "Key values should be swapped HERE";
        let macro_value = "when defined on record";
        init(0);
        let buffer = buffer();
        buffer.lock().unwrap().start_logging();
        info!("{}", txt; "HERE" => macro_value);
        let result = format!("{}", buffer.lock().unwrap().to_string());
        assert!(result.contains(macro_value));
    }
}
