// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use git_version;

fn main() {
    git_version::set_env();

    // Run in single thread due the active/inactive tests not
    // supporting to run in parallel for now.
    println!("cargo:rustc-env=RUST_TEST_THREADS=1");
}
