/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package metadata

const (
	ValidJSONMetadata = `{
	  "product-uid": "0123456789",
	  "supported-hardware": [
	    {"hardware": "hardware1", "hardware-revision": "revA"},
	    {"hardware": "hardware2", "hardware-revision": "revB"}
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
)
