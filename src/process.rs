// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: GPL-2.0
// 

use checked_command;
use cmdline_words_parser::StrExt;

pub fn run(cmd: &str) -> Result<checked_command::Output, checked_command::Error> {
    let mut cmd = cmd.to_string();
    let mut cmd = cmd.parse_cmdline_words();

    let mut p = checked_command::CheckedCommand::new(cmd.next().unwrap());
    for arg in cmd {
        p.arg(arg);
    }

    Ok(p.output()?)
}

#[test]
fn stdout() {
    // stdout
    let output = run(r#"sh -c 'echo "1 2 3 4"'"#).unwrap();
    assert_eq!(String::from_utf8_lossy(&output.stdout), "1 2 3 4\n");
}

#[test]
fn stderr() {
    // stderr
    let output = run(r#"sh -c 'echo "1 2 3 4" >&2'"#).unwrap();
    assert_eq!(String::from_utf8_lossy(&output.stderr), "1 2 3 4\n");
}

#[test]
fn failing_command() {
    // failing command with exit status 1
    let r = run(r#"sh -c 'echo "error" >&2; exit 1'"#);
    match r {
        Ok(_) => panic!("call should have failed"),
        Err(checked_command::Error::Io(io_err)) => panic!("unexpected I/O Error: {:?}", io_err),
        Err(checked_command::Error::Failure(ex, output)) => {
            assert_eq!(ex.code().unwrap(), 1);
            assert_eq!(String::from_utf8_lossy(&output.unwrap().stderr), "error\n");
        }
    }
}
