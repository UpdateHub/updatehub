/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package metadata

const (
	ValidJSONMetadata = `{
	  "product-uid": "0123456789",
	  "supported-hardware": [
	    "hardware1-revA",
	    "hardware2-revB"
	  ],
	  "objects": [
	    [
	      { "mode": "test" }
	    ]
	  ]
	}`

	ValidJSONMetadataWithActiveInactive = `{
	  "product-uid": "0123456789",
	  "objects": [
	    [
	      {
            "mode": "test",
            "target": "/dev/xx1",
            "target-type": "device"
          }
	    ]
        ,
	    [
	      {
            "mode": "test",
            "target": "/dev/xx2",
            "target-type": "device"
          }
	    ]
	  ]
	}`

	ValidJSONMetadataWithCompressedObject = `{
	  "product-uid": "0123456789",
	  "objects": [
	    [
	      { "mode": "compressed-object", "compressed": true }
	    ]
	  ]
	}`

	ValidJSONMetadataWithoutCompressedObject = `{
	  "product-uid": "0123456789",
	  "objects": [
	    [
	      { "mode": "test", "compressed": true }
	    ]
	  ]
	}`

	ValidJSONMetadataWithSupportedHardwareAny = `{
	  "product-uid": "0123456789",
	  "supported-hardware": "any",
	  "objects": [
	    [
	      { "mode": "test" }
	    ]
	  ]
	}`

	ValidJSONMetadataWithUnknownSupportedHardwareFormat = `{
	  "product-uid": "0123456789",
	  "supported-hardware": { "hardware": "h1-revA" },
	  "objects": [
	    [
	      { "mode": "test" }
	    ]
	  ]
	}`
)
