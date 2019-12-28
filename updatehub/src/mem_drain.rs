// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use serde::Serialize;
use slog::{Drain, Key, OwnedKVList, Record, KV};
use std::{
    collections::HashMap,
    fmt::{self, Write},
    io,
    sync::Mutex,
};

#[derive(Debug, Default)]
pub struct MemDrain {
    records: Mutex<Vec<LogRecord>>,
    logging: bool,
}

#[derive(Debug, Serialize)]
struct LogRecord {
    level: String,
    message: String,
    time: String,
    data: HashMap<String, String>,
}

impl MemDrain {
    pub fn clear(&self) {
        self.records.lock().unwrap().clear();
    }

    pub fn start_logging(&mut self) {
        self.logging = true;
    }

    pub fn stop_logging(&mut self) {
        self.logging = false;
    }
}

impl Serialize for MemDrain {
    fn serialize<S>(&self, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        self.records.serialize(serializer)
    }
}

impl ToString for MemDrain {
    fn to_string(&self) -> String {
        let records = self.records.lock().unwrap();

        let mut ret = String::new();
        for record in records.iter() {
            let mut msg = record.message.clone();
            for (k, v) in &record.data {
                msg = msg.replace(k, v);
            }

            writeln!(&mut ret, "{} {} {}", record.time, record.level, msg).unwrap();
        }
        ret
    }
}

impl Drain for MemDrain {
    type Err = io::Error;
    type Ok = ();

    fn log(&self, record: &Record, kvs: &OwnedKVList) -> io::Result<()> {
        if self.logging {
            let mut kv = KVSerializer::default();
            record.kv().serialize(record, &mut kv)?;
            kvs.serialize(record, &mut kv)?;

            let l = LogRecord {
                level: record.level().as_str().to_lowercase(),
                message: fmt::format(*record.msg()),
                time: chrono::Local::now().format("%F %H:%M:%S%.9f %z").to_string(),
                data: kv.0,
            };

            self.records.lock().unwrap().push(l);
        }

        Ok(())
    }
}

#[derive(Default)]
struct KVSerializer(HashMap<String, String>);

impl slog::ser::Serializer for KVSerializer {
    fn emit_arguments(&mut self, key: Key, val: &fmt::Arguments) -> slog::Result {
        let val = &format!("{:?}", val);
        self.0.insert(key.to_string(), val.to_string());
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use slog::{o, slog_debug, slog_error, slog_info, Logger};
    use std::sync::Arc;

    fn eq_without_time(s1: &str, s2: &str) -> bool {
        let s1 = s1.split('\n');
        let s2 = s2.split('\n');
        for (i, (x, y)) in s1.zip(s2).enumerate() {
            if x.contains("time") {
                continue;
            }
            if x != y {
                println!("Difference on string's line: {}\n{} != {}", i, x, y);
                return false;
            }
        }
        true
    }

    #[test]
    fn drain_storage_log() {
        let s1 = "Multiple log messages should";
        let s2 = "all be find inside log string";
        let drain = Arc::new(Mutex::new(MemDrain::default()));
        let r_vec = drain.clone();
        drain.lock().unwrap().start_logging();
        let log = Logger::root(drain.fuse(), o!());
        slog_info!(log, "{}", s1);
        slog_info!(log, "{}", s2);
        let result = r_vec.lock().unwrap().to_string();
        println!("{}", result);
        assert!(result.contains(s1));
        assert!(result.contains(s2));
    }

    #[test]
    fn drain_format() {
        let s1 = "Log should contain message type";
        let s2 = "Type strings are shorten";
        let drain = Arc::new(Mutex::new(MemDrain::default()));
        let r_vec = drain.clone();
        drain.lock().unwrap().start_logging();
        let log = Logger::root(drain.fuse(), o!());
        slog_info!(log, "{}", s1);
        slog_debug!(log, "{}", s2);
        let result = r_vec.lock().unwrap().to_string();
        println!("{}", result);
        assert!(result.contains("info"));
        assert!(result.contains("debug"));
    }

    #[test]
    fn drain_key_values() {
        let txt = "Key values should be swapped, LOGGER and RECORD";
        let logger_value = "when defined on logger";
        let macro_value = "when defined on record";
        let drain = Arc::new(Mutex::new(MemDrain::default()));
        let r_vec = drain.clone();
        drain.lock().unwrap().start_logging();
        let log = Logger::root(drain.fuse(), o!("LOGGER" => logger_value));
        slog_info!(log, "{}", txt; "RECORD" => macro_value);
        let result = r_vec.lock().unwrap().to_string();
        println!("{}", result);
        assert!(result.contains(logger_value));
        assert!(result.contains(macro_value));
    }

    #[test]
    fn drain_serialized() {
        let expected = r#"[
  {
    "level": "info",
    "message": "info 1",
    "time": "2017-06-29 13:59:31.831111065 -0300 -03",
    "data": {}
  },
  {
    "level": "info",
    "message": "info 2",
    "time": "2017-06-29 14:59:41.831111065 -0300 -03",
    "data": {
      "field1": "value1"
    }
  },
  {
    "level": "error",
    "message": "error n",
    "time": "2017-06-29 15:59:51.831111065 -0300 -03",
    "data": {}
  }
]"#;

        let drain = Arc::new(Mutex::new(MemDrain::default()));
        let r_vec = drain.clone();
        drain.lock().unwrap().start_logging();
        let log = Logger::root(drain.fuse(), o!());
        slog_info!(log, "{}", "info 1");
        slog_info!(log, "{}", "info 2"; "field1" => "value1");
        slog_error!(log, "{}", "error n");
        let result = serde_json::to_string_pretty(&r_vec).unwrap();
        assert!(
            eq_without_time(&expected, &result),
            format!("Expected:\n{}\n\nResult:\n{}", expected, result)
        );
    }
}
