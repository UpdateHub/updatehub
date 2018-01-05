//
// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: GPL-2.0
//

use serde::Serializer;

pub fn bool_to_string<S>(v: &bool, serializer: S) -> Result<S::Ok, S::Error>
    where S: Serializer {
    Ok(serializer.serialize_str(if *v { "true" } else { "false" })?)
}
