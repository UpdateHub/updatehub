// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: GPL-2.0-only
// 

//! Build information module

const VERSION: &str = env!("VERSION");

/// Returns the version in use, including the commit and if there is
/// uncommited modification in the source.
///
/// Internally, it use `git describe` to get the version and the
/// number of changes since the last tag.
///
/// # Example
/// ```
/// use updatehub::build_info;
///
/// println!("Running version: {}", build_info::version());
/// ```
pub fn version() -> &'static str {
    VERSION
}
