// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
// 

use super::*;
use serde_json;

pub fn get_update_json() -> serde_json::Value {
    json!(
        {
            "product-uid": "0123456789",
            "version": "1.0",
            "supported-hardware": "board",
            "objects":
            [
                {
                    "mode": "test",
                    "filename": "testfile",
                    "target": "/dev/device1",
                    "sha256sum": "xxx"
                }
            ]
        }
    )
}

pub fn get_update_package() -> UpdatePackage {
    let json = get_update_json();

    serde_json::from_value(json).unwrap()
}
