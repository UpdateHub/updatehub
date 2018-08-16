// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
//

macro_rules! impl_object_for_object_types {
    ( $( $objtype:ident ),* ) => {
        impl Object {
            pub fn status(&self, download_dir: &Path) -> Result<ObjectStatus> {
                match *self {
                    $( Object::$objtype(ref o) => Ok(o.status(download_dir)?), )*
                }
            }

            pub fn filename(&self) -> &str {
                match *self {
                    $( Object::$objtype(ref o) => o.filename(), )*
                }
            }

            pub fn len(&self) -> u64 {
                match *self {
                    $( Object::$objtype(ref o) => o.len(), )*
                }
            }

            pub fn sha256sum(&self) -> &str {
                match *self {
                    $( Object::$objtype(ref o) => o.sha256sum(), )*
                }
            }
        }
    };
}

macro_rules! impl_object_type {
    ($objtype:ident) => {
        impl ObjectType for $objtype {
            fn filename(&self) -> &str {
                &self.filename
            }

            fn len(&self) -> u64 {
                self.size
            }

            fn sha256sum(&self) -> &str {
                &self.sha256sum
            }
        }
    };
}
