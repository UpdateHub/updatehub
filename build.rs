// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
// 

extern crate git_version;

fn main() {
    git_version::set_env();

    // This forces the tests to run in a single thread. This is
    // required for use of the mock server[1] otherwise one test impacts
    // the other.
    //
    // 1. https://github.com/lipanski/mockito/issues/25
    println!("cargo:rustc-env=RUST_TEST_THREADS=1");
}
