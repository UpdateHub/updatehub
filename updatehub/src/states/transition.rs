// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::Result;
use slog_scope::error;
use std::path::Path;

const STATE_CHANGE_CALLBACK: &str = "state-change-callback";

#[derive(Debug, PartialEq)]
pub(super) enum Transition {
    Continue,
    Cancel,
}

pub(super) fn state_change_callback(path: &Path, state: &'static str) -> Result<Transition> {
    use std::io;

    let callback = path.join(STATE_CHANGE_CALLBACK);
    if !callback.exists() {
        return Ok(Transition::Continue);
    }

    let output = easy_process::run(&format!("{} {}", &callback.to_string_lossy(), &state))?;
    for err in output.stderr.lines() {
        error!("{} (stderr): {}", path.display(), err);
    }

    match output.stdout.trim() {
        "cancel" => Ok(Transition::Cancel),
        "" => Ok(Transition::Continue),
        _ => Err(io::Error::new(
            io::ErrorKind::InvalidInput,
            format!(
                "Invalid format found while running 'state-change-callback' \
                 hook for state '{}'",
                &state
            ),
        )
        .into()),
    }
}

#[cfg(test)]
mod test {
    use super::*;
    use pretty_assertions::assert_eq;
    use tempfile;

    const CALLBACK_STATE_NAME: &str = "test_state";

    fn create_state_change_callback_hook(content: &str) -> tempfile::TempDir {
        use crate::firmware::tests::create_hook;

        let tmpdir = tempfile::tempdir().unwrap();
        let tmpdir = tmpdir;

        create_hook(tmpdir.path().join(STATE_CHANGE_CALLBACK), content);
        tmpdir
    }

    #[test]
    fn cancel() {
        let script = "#!/bin/sh\necho cancel";
        let tmpdir = create_state_change_callback_hook(&script);
        assert_eq!(
            state_change_callback(&tmpdir.path(), CALLBACK_STATE_NAME).unwrap(),
            Transition::Cancel,
            "Unexpected result using content {:?}",
            script,
        );
    }

    #[test]
    fn continue_transition() {
        let script = "#!/bin/sh\necho ";
        let tmpdir = create_state_change_callback_hook(&script);
        assert_eq!(
            state_change_callback(&tmpdir.path(), CALLBACK_STATE_NAME).unwrap(),
            Transition::Continue,
            "Unexpected result using content {:?}",
            script,
        );
    }

    #[test]
    fn non_existing_hook() {
        assert_eq!(
            state_change_callback(&Path::new("/NaN"), CALLBACK_STATE_NAME).unwrap(),
            Transition::Continue,
            "Unexpected result for non-existing hook",
        );
    }

    #[test]
    fn is_error() {
        for script in &["#!/bin/sh\necho 123", "#!/bin/sh\necho 123\ncancel"] {
            let tmpdir = create_state_change_callback_hook(script);
            assert!(state_change_callback(&tmpdir.path(), CALLBACK_STATE_NAME).is_err());
        }
    }
}
