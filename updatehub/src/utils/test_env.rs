// Copyright (C) 2026 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

//! Use `super::fs::search_path::SearchPathGuard` instead whenever the test
//! only needs `is_executable_in_path` to see other directories. It changes no
//! process state at all.

use std::{
    env,
    ffi::OsString,
    path::Path,
    sync::{LazyLock, Mutex, MutexGuard},
};

static ENV_LOCK: LazyLock<Mutex<()>> = LazyLock::new(Mutex::default);

/// Holds `PATH` at a test-defined value, and restores it on drop.
///
/// The lock is not reentrant: a test that needs two values must drop the
/// first guard before it creates the second.
#[must_use = "PATH goes back to its previous value as soon as the guard drops"]
pub(crate) struct PathEnvGuard {
    _lock: MutexGuard<'static, ()>,
    previous: Option<OsString>,
}

impl PathEnvGuard {
    /// Replaces `PATH` with `value` until the guard drops.
    #[cfg(test)]
    pub(crate) fn set(value: impl Into<OsString>) -> Self {
        let (lock, previous) = Self::acquire();
        // SAFETY: `acquire` returned the lock.
        unsafe { Self::write(Some(value.into())) };

        PathEnvGuard { _lock: lock, previous }
    }

    /// Puts `dir` in front of the current `PATH` until the guard drops.
    pub(crate) fn prepend(dir: &Path) -> Self {
        let (lock, previous) = Self::acquire();

        let mut value = OsString::from(dir);
        if let Some(current) = previous.as_ref().filter(|p| !p.is_empty()) {
            value.push(":");
            value.push(current);
        }
        // SAFETY: `acquire` returned the lock.
        unsafe { Self::write(Some(value)) };

        PathEnvGuard { _lock: lock, previous }
    }

    /// Reads the value to restore under the lock, so no other thread changes
    /// `PATH` between the read and the write.
    fn acquire() -> (MutexGuard<'static, ()>, Option<OsString>) {
        // A panicking test poisons the lock, but `Drop` still restored `PATH`
        // during the unwind, so the poison carries no information.
        let lock = ENV_LOCK.lock().unwrap_or_else(|poisoned| poisoned.into_inner());
        let previous = env::var_os("PATH");

        (lock, previous)
    }

    /// # Safety
    ///
    /// The caller must hold `ENV_LOCK`.
    unsafe fn write(value: Option<OsString>) {
        match value {
            Some(value) => unsafe { env::set_var("PATH", value) },
            None => unsafe { env::remove_var("PATH") },
        }
    }
}

impl Drop for PathEnvGuard {
    fn drop(&mut self) {
        // SAFETY: the guard still owns the lock.
        unsafe { Self::write(self.previous.take()) };
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use pretty_assertions::assert_eq;

    #[test]
    fn set_restores_the_previous_path_on_drop() {
        let before = env::var_os("PATH");

        let guard = PathEnvGuard::set("/updatehub-fake-dir");
        assert_eq!(env::var_os("PATH"), Some(OsString::from("/updatehub-fake-dir")));
        drop(guard);

        assert_eq!(env::var_os("PATH"), before);
    }

    #[test]
    fn prepend_keeps_the_previous_path_behind_the_new_directory() {
        let before = env::var_os("PATH").expect("the test host has to define PATH");

        let guard = PathEnvGuard::prepend(Path::new("/updatehub-fake-dir"));
        let mut expected = OsString::from("/updatehub-fake-dir:");
        expected.push(&before);
        assert_eq!(env::var_os("PATH"), Some(expected));
        drop(guard);

        assert_eq!(env::var_os("PATH"), Some(before));
    }
}
