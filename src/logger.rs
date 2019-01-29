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
