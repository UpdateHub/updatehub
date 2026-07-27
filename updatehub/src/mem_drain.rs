// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use serde::Serialize;
use slog::{Drain, KV, Key, OwnedKVList, Record};
use std::{
    borrow::Cow,
    collections::{HashMap, VecDeque},
    fmt::{self, Display},
    io,
    sync::{RwLock, RwLockReadGuard, RwLockWriteGuard},
};

/// Upper bound for the memory held by the recorded entries.
///
/// This is a backstop rather than an operational limit: recording is scoped to
/// a single update operation, which never comes close to it. It is here so that
/// a state machine looping unexpectedly, as an offline device retrying to probe
/// does, cannot grow the buffer without bound.
const MAX_RECORDED_BYTES: usize = 1024 * 1024;

#[derive(Debug, Default)]
pub struct MemDrain {
    records: RwLock<Records>,
    logging: bool,
}

#[derive(Debug, Default)]
struct Records {
    entries: VecDeque<LogRecord>,
    /// Sum of `LogRecord::size` over `entries`, kept up to date on insertion
    /// and eviction so enforcing the bound does not require walking the
    /// entries.
    bytes: usize,
    /// How many entries were ever inserted. Never reset, not even by `clear`,
    /// so that `first_index` grows monotonically and a reader can tell
    /// entries apart across both eviction and the start of a new operation.
    inserted: usize,
}

#[derive(Debug)]
struct LogRecord {
    level: String,
    message: String,
    time: String,
    data: HashMap<String, String>,
    /// How many times this record was logged in a row. Consecutive repeats are
    /// counted here instead of being stored as separate entries.
    count: usize,
}

impl Records {
    /// Absolute index of the oldest entry still held.
    fn first_index(&self) -> usize {
        self.inserted - self.entries.len()
    }

    fn clear(&mut self) {
        self.entries.clear();
        self.bytes = 0;
    }

    fn push(&mut self, record: LogRecord) {
        // A device unable to reach the server repeats the very same failure on
        // every retry; counting those keeps both the log readable and the
        // memory it takes constant.
        if let Some(last) = self.entries.back_mut() {
            if last.repeats(&record) {
                last.count += 1;
                return;
            }
        }

        self.bytes += record.size();
        self.entries.push_back(record);
        self.inserted += 1;

        // Always keep the newest entry, even if it alone exceeds the bound.
        while self.bytes > MAX_RECORDED_BYTES && self.entries.len() > 1 {
            if let Some(evicted) = self.entries.pop_front() {
                self.bytes -= evicted.size();
            }
        }
    }
}

impl LogRecord {
    fn size(&self) -> usize {
        std::mem::size_of::<Self>()
            + self.level.len()
            + self.message.len()
            + self.time.len()
            + self.data.iter().map(|(k, v)| k.len() + v.len()).sum::<usize>()
    }

    fn repeats(&self, other: &Self) -> bool {
        self.level == other.level && self.message == other.message && self.data == other.data
    }

    /// The message as presented to a reader, reporting any counted repeats.
    fn message(&self) -> Cow<'_, str> {
        if self.count > 1 {
            Cow::Owned(format!("{} (repeated {} times)", self.message, self.count))
        } else {
            Cow::Borrowed(&self.message)
        }
    }
}

impl Serialize for LogRecord {
    fn serialize<S>(&self, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;

        let mut state = serializer.serialize_struct("Entry", 4)?;
        state.serialize_field("level", &self.level)?;
        state.serialize_field("message", &self.message())?;
        state.serialize_field("time", &self.time)?;
        state.serialize_field("data", &self.data)?;
        state.end()
    }
}

impl MemDrain {
    /// The recorded entries, recovering the lock if a thread panicked while
    /// holding it: being unable to log must not bring the agent down.
    fn records(&self) -> RwLockReadGuard<'_, Records> {
        self.records.read().unwrap_or_else(|poisoned| poisoned.into_inner())
    }

    fn records_mut(&self) -> RwLockWriteGuard<'_, Records> {
        self.records.write().unwrap_or_else(|poisoned| poisoned.into_inner())
    }

    /// Start recording a new operation, discarding what the previous one left.
    pub fn start_logging(&mut self) {
        self.records_mut().clear();
        self.logging = true;
    }

    /// Stop recording. Entries already recorded are kept, so the last operation
    /// remains available for reading.
    pub fn stop_logging(&mut self) {
        self.logging = false;
    }

}

impl Serialize for MemDrain {
    fn serialize<S>(&self, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;

        let records = self.records();

        let mut state = serializer.serialize_struct("Log", 2)?;
        state.serialize_field("entries", &records.entries)?;
        state.serialize_field("first_index", &records.first_index())?;
        state.end()
    }
}

impl Display for MemDrain {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let records = self.records();

        for record in &records.entries {
            let mut msg = record.message().into_owned();
            for (k, v) in &record.data {
                msg = msg.replace(k, v);
            }

            writeln!(f, "{} {} {}", record.time, record.level, msg)?;
        }

        Ok(())
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
                time: chrono::Local::now().format("%b %d %H:%M:%S%.3f").to_string(),
                data: kv.0,
                count: 1,
            };

            self.records_mut().push(l);
        }

        Ok(())
    }
}

#[derive(Default)]
struct KVSerializer(HashMap<String, String>);

impl slog::ser::Serializer for KVSerializer {
    fn emit_arguments(&mut self, key: Key, val: &fmt::Arguments) -> slog::Result {
        let val = &format!("{val:?}");
        self.0.insert(key.to_string(), val.to_string());
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use slog::{Logger, o, slog_debug, slog_error, slog_info};
    use std::sync::{Arc, Mutex};

    fn eq_without_time(s1: &str, s2: &str) -> bool {
        let s1 = s1.split('\n');
        let s2 = s2.split('\n');
        for (i, (x, y)) in s1.zip(s2).enumerate() {
            if x.contains("time") {
                continue;
            }
            if x != y {
                println!("Difference on string's line: {i}\n{x} != {y}");
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
        println!("{result}");
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
        println!("{result}");
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
        println!("{result}");
        assert!(result.contains(logger_value));
        assert!(result.contains(macro_value));
    }

    #[test]
    fn drain_serialized() {
        let expected = r#"{
  "entries": [
    {
      "level": "info",
      "message": "info 1",
      "time": "Aug 27 16:09:48.740",
      "data": {}
    },
    {
      "level": "info",
      "message": "info 2",
      "time": "Aug 27 16:09:48.740",
      "data": {
        "field1": "value1"
      }
    },
    {
      "level": "error",
      "message": "error n",
      "time": "Aug 27 16:09:48.740",
      "data": {}
    }
  ],
  "first_index": 0
}"#;

        let drain = Arc::new(Mutex::new(MemDrain::default()));
        let r_vec = drain.clone();
        drain.lock().unwrap().start_logging();
        let log = Logger::root(drain.fuse(), o!());
        slog_info!(log, "{}", "info 1");
        slog_info!(log, "{}", "info 2"; "field1" => "value1");
        slog_error!(log, "{}", "error n");
        let result = serde_json::to_string_pretty(&r_vec).unwrap();
        assert!(eq_without_time(expected, &result), "Expected:\n{expected}\n\nResult:\n{result}");
    }

    #[test]
    fn drain_evicts_entries_to_stay_bounded() {
        let drain = Arc::new(Mutex::new(MemDrain::default()));
        let handle = drain.clone();
        drain.lock().unwrap().start_logging();
        let log = Logger::root(drain.fuse(), o!());

        // Distinct messages, so none of them can be counted as a repeat.
        let logged = 20_000;
        for i in 0..logged {
            slog_error!(log, "Probe failed: could not reach the server, attempt {}", i);
        }

        let drain = handle.lock().unwrap();
        let records = drain.records.read().unwrap();
        assert!(
            records.bytes <= MAX_RECORDED_BYTES,
            "recorded {} bytes, past the {MAX_RECORDED_BYTES} bytes bound",
            records.bytes
        );
        assert!(records.entries.len() < logged, "nothing was evicted");
        assert_eq!(records.inserted, logged);
        assert_eq!(records.first_index(), logged - records.entries.len());
    }

    #[test]
    fn drain_counts_repeated_records() {
        let drain = Arc::new(Mutex::new(MemDrain::default()));
        let handle = drain.clone();
        drain.lock().unwrap().start_logging();
        let log = Logger::root(drain.fuse(), o!());

        // What an offline device produces, once per probe retry.
        for _ in 0..3_600 {
            slog_error!(log, "Probe failed: {}", "dns error");
        }

        let result = handle.lock().unwrap().to_string();
        assert_eq!(handle.lock().unwrap().records.read().unwrap().entries.len(), 1);
        assert!(result.contains("(repeated 3600 times)"), "unexpected log:\n{result}");
    }

    #[test]
    fn drain_advances_first_index_on_new_operation() {
        let drain = Arc::new(Mutex::new(MemDrain::default()));
        let handle = drain.clone();
        drain.lock().unwrap().start_logging();
        let log = Logger::root(drain.fuse(), o!());

        slog_info!(log, "{}", "first operation");
        assert_eq!(handle.lock().unwrap().records.read().unwrap().first_index(), 0);

        // Recording a new operation drops the previous entries, but a reader
        // must not take the ones that follow for the ones it already read.
        handle.lock().unwrap().start_logging();
        slog_info!(log, "{}", "second operation");

        let drain = handle.lock().unwrap();
        let records = drain.records.read().unwrap();
        assert_eq!(records.entries.len(), 1);
        assert_eq!(records.first_index(), 1);
    }
}
