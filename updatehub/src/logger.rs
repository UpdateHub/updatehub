// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::mem_drain::MemDrain;
use slog::{Drain, Logger, o};
use std::sync::{Arc, LazyLock, Mutex, MutexGuard};

static BUFFER: LazyLock<Arc<Mutex<MemDrain>>> =
    LazyLock::new(|| Arc::new(Mutex::new(MemDrain::default())));

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

/// The buffered drain, recovering the lock if a thread panicked while holding
/// it: being unable to log must not bring the agent down.
fn buffer_lock() -> MutexGuard<'static, MemDrain> {
    BUFFER.lock().unwrap_or_else(std::sync::PoisonError::into_inner)
}

pub fn start_memory_logging() {
    buffer_lock().start_logging();
}

pub fn stop_memory_logging() {
    buffer_lock().stop_logging();
}

/// Record what `f` logs even while memory logging is stopped.
///
/// Memory logging is scoped to update activity, but a device failing to reach
/// the server has no update to report and its failure is the only evidence it
/// can offer, so that one is kept regardless of scope. The current scope is
/// preserved: this never discards what was already recorded.
pub fn record_out_of_scope<R>(f: impl FnOnce() -> R) -> R {
    let was_logging = buffer_lock().is_logging();

    // The lock must not be held across `f`, as recording takes it again.
    buffer_lock().set_logging(true);
    let result = f();
    buffer_lock().set_logging(was_logging);

    result
}

/// Returns everything the memory drain holds for the current scope of
/// operation.
#[must_use]
pub fn get_memory_log() -> String {
    buffer_lock().to_string()
}
