// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use cloud::{api, Error, Result};
use std::{cell::RefCell, marker::PhantomData, path::Path};

std::thread_local! {
    static RESPONSE_CONFIG: RefCell<FakeResponse> = RefCell::new(FakeResponse::NoUpdate);
}

std::thread_local! {
    static OBJECT_DATA: RefCell<Option<Vec<u8>>> = RefCell::new(Option::None);
}

pub(crate) enum FakeResponse {
    NoUpdate,
    HasUpdate,
    ExtraPoll,
    InvalidUri,
}

pub(crate) struct Client<'a> {
    _phantom: PhantomData<&'a ()>,
}

pub(crate) fn setup_fake_response(res: FakeResponse) {
    RESPONSE_CONFIG.with(|conf| conf.replace_with(move |&mut _| res));
}

pub(crate) fn set_download_data(data: Vec<u8>) {
    OBJECT_DATA.with(|conf| conf.borrow_mut().replace(data));
}

impl<'a> Client<'a> {
    pub(crate) fn new(_server: &'a str) -> Self {
        Self { _phantom: PhantomData }
    }

    pub(crate) async fn probe(
        &self,
        _num_retries: usize,
        _firmware: api::FirmwareMetadata<'_>,
    ) -> Result<api::ProbeResponse> {
        RESPONSE_CONFIG.with(|conf| match std::ops::Deref::deref(&conf.borrow()) {
            FakeResponse::NoUpdate => Ok(api::ProbeResponse::NoUpdate),
            FakeResponse::ExtraPoll => Ok(api::ProbeResponse::ExtraPoll(10)),
            FakeResponse::HasUpdate => Ok(api::ProbeResponse::Update(
                crate::update_package::tests::get_update_package(),
                None,
            )),
            FakeResponse::InvalidUri => {
                let uri_error = url::Url::parse("http://foo:--").unwrap_err();
                Err(Error::UrlParse(uri_error))
            }
        })
    }

    pub(crate) async fn download_object(
        &self,
        _product_uid: &str,
        _package_uid: &str,
        download_dir: &Path,
        object: &str,
    ) -> Result<()> {
        if let Some(data) = OBJECT_DATA.with(|conf| conf.borrow_mut().take()) {
            tokio::fs::write(download_dir.join(object), data).await?
        }

        Ok(())
    }

    pub(crate) async fn report(
        &self,
        _state: &str,
        _firmware: api::FirmwareMetadata<'_>,
        _package_uid: &str,
        _previous_state: Option<&str>,
        _error_message: Option<String>,
        _current_log: Option<String>,
    ) -> Result<()> {
        Ok(())
    }
}
