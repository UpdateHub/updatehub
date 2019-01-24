// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use slog::{Drain, Key, OwnedKVList, Record, KV};
use std::{fmt, io, sync::Mutex};

#[derive(Debug)]
pub struct MemDrain {
    buffer: Mutex<Vec<String>>,
    logging: bool,
}

impl MemDrain {
    pub fn new() -> Self {
        let buffer = Mutex::new(vec![]);
        let logging = false;
        MemDrain { buffer, logging }
    }

    pub fn clear(&self) {
        self.buffer.lock().unwrap().clear();
    }

    pub fn start_logging(&mut self) {
        self.logging = true;
    }

    pub fn stop_logging(&mut self) {
        self.logging = false;
    }

    pub fn to_string(&self) -> String {
        let buffer = self.buffer.lock().unwrap();
        buffer.join("\n")
    }
}

impl Drain for MemDrain {
    type Ok = ();
    type Err = io::Error;

    fn log(&self, record: &Record, kvs: &OwnedKVList) -> io::Result<()> {
        if self.logging {
            let mut buffer = self.buffer.lock().unwrap();
            const TIMESTAMP_FORMAT: &'static str = "%b %d %H:%M:%S%.3f";
            let mut serializer = Serializer::new(fmt::format(*record.msg()));

            record.kv().serialize(record, &mut serializer)?;
            kvs.serialize(record, &mut serializer)?;
            let line = format!(
                "{} {} {}",
                chrono::Local::now().format(TIMESTAMP_FORMAT),
                record.level().as_short_str(),
                serializer.text,
            );

            buffer.push(line);
        }
        Ok(())
    }
}

struct Serializer {
    text: String,
}

impl Serializer {
    fn new(text: String) -> Self {
        Serializer { text }
    }
}

impl slog::ser::Serializer for Serializer {
    fn emit_arguments(&mut self, key: Key, val: &fmt::Arguments) -> slog::Result {
        self.text = self.text.replace(key, &format!("{:?}", val));
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use slog::{o, slog_debug, slog_info, Logger};
    use std::sync::Arc;

    #[test]
    fn drain_storage_log() {
        let s1 = "Multiple log messages should";
        let s2 = "all be find inside log string";
        let drain = Arc::new(Mutex::new(MemDrain::new()));
        let r_vec = drain.clone();
        drain.lock().unwrap().start_logging();
        let log = Logger::root(drain.fuse(), o!());
        slog_info!(log, "{}", s1);
        slog_info!(log, "{}", s2);
        let result = format!("{}", r_vec.lock().unwrap().to_string());
        assert!(result.contains(s1));
        assert!(result.contains(s2));
    }

    #[test]
    fn drain_format() {
        let s1 = "Log should contain message type";
        let s2 = "Type strings are shorten";
        let drain = Arc::new(Mutex::new(MemDrain::new()));
        let r_vec = drain.clone();
        drain.lock().unwrap().start_logging();
        let log = Logger::root(drain.fuse(), o!());
        slog_info!(log, "{}", s1);
        slog_debug!(log, "{}", s2);
        let result = format!("{}", r_vec.lock().unwrap().to_string());
        assert!(result.contains("INFO"));
        assert!(result.contains("DEBG"));
    }

    #[test]
    fn drain_key_values() {
        let txt = "Key values should be swapped, LOGGER and RECORD";
        let logger_value = "when defined on logger";
        let macro_value = "when defined on record";
        let drain = Arc::new(Mutex::new(MemDrain::new()));
        let r_vec = drain.clone();
        drain.lock().unwrap().start_logging();
        let log = Logger::root(drain.fuse(), o!("LOGGER" => logger_value));
        slog_info!(log, "{}", txt; "RECORD" => macro_value);
        let result = format!("{}", r_vec.lock().unwrap().to_string());
        assert!(result.contains(logger_value));
        assert!(result.contains(macro_value));
    }
}
