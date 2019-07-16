// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

macro_rules! impl_object_for_object_types {
    ( $( $objtype:ident ),* ) => {
        impl Object {
            pub(crate) fn status(&self, download_dir: &std::path::Path) -> Result<ObjectStatus, failure::Error> {
                match *self {
                    $( Object::$objtype(ref o) => Ok(o.status(download_dir)?), )*
                    #[cfg(test)]
                    Object::Test(ref o) => Ok(o.status(download_dir)?),
                }
            }

            pub(crate) fn filename(&self) -> &str {
                match *self {
                    $( Object::$objtype(ref o) => o.filename(), )*
                    #[cfg(test)]
                    Object::Test(ref o) => o.filename(),
                }
            }

            pub(crate) fn len(&self) -> u64 {
                match *self {
                    $( Object::$objtype(ref o) => o.len(), )*
                    #[cfg(test)]
                    Object::Test(ref o) => o.len(),
                }
            }

            pub(crate) fn sha256sum(&self) -> &str {
                match *self {
                    $( Object::$objtype(ref o) => o.sha256sum(), )*
                    #[cfg(test)]
                    Object::Test(ref o) => o.sha256sum(),
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

macro_rules! for_any_object {
    ($mode:ident, $alias:ident, $code:block) => {
        match $mode {
            Object::Copy($alias) => $code,
            Object::Raw($alias) => $code,
            #[cfg(test)]
            Object::Test($alias) => $code,
            _ => panic!("FIXME: TODO: Mode still not supported"),
        }
    };
}
