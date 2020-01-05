// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use git_version::git_version;

fn main() {
    println!("cargo:rustc-env=VERSION={}", git_version!());

    // Run in single thread due the active/inactive tests not
    // supporting to run in parallel for now.
    println!("cargo:rustc-env=RUST_TEST_THREADS=1");
}
