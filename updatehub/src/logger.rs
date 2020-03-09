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
    static ref BUFFER: Arc<Mutex<MemDrain>> = Arc::new(Mutex::new(MemDrain::default()));
}

pub fn init(level: slog::Level) {
    let buffer_drain = buffer().filter_level(level).fuse();
    let terminal_drain = Mutex::new(slog_term::term_full().filter_level(level)).fuse();
    let terminal_drain = slog_async::Async::new(terminal_drain).build().fuse();

    let log = Logger::root(slog::Duplicate::new(buffer_drain, terminal_drain).fuse(), o!());

    // FIXME: Drop the use of Box::leak here (issue #23).
    let guard = slog_scope::set_global_logger(log);
    Box::leak(Box::new(guard));
}

pub fn buffer() -> Arc<Mutex<MemDrain>> {
    BUFFER.clone()
}

pub fn start_memory_logging() {
    BUFFER.lock().unwrap().start_logging()
}

pub fn stop_memory_logging() {
    BUFFER.lock().unwrap().stop_logging()
}

pub fn get_memory_log() -> String {
    BUFFER.lock().unwrap().to_string()
}
