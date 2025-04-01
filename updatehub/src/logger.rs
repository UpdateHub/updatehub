// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::mem_drain::MemDrain;
use lazy_static::lazy_static;
use slog::{Drain, Logger, o};
use std::sync::{Arc, Mutex};

lazy_static! {
    static ref BUFFER: Arc<Mutex<MemDrain>> = Arc::new(Mutex::new(MemDrain::default()));
}

pub fn init(level: slog::Level) -> slog_scope::GlobalLoggerGuard {
    let buffer_drain = buffer().filter_level(level).fuse();
    let terminal_drain = Mutex::new(
        slog_term::FullFormat::new(slog_term::TermDecorator::new().force_plain().build())
            .build()
            .filter_level(level),
    )
    .fuse();
    let terminal_drain = slog_async::Async::new(terminal_drain).build().fuse();

    let log = Logger::root(slog::Duplicate::new(buffer_drain, terminal_drain).fuse(), o!());

    slog_scope::set_global_logger(log)
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
