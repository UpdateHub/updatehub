// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::states::machine;
use sdk::api;
use slog_scope::debug;
use std::convert::TryFrom;

pub(crate) struct Api(machine::Addr);

impl Api {
    pub(crate) fn server(addr: machine::Addr) -> tide::Server<machine::Addr> {
        let mut server = tide::with_state(addr);
        server.at("/info").get(Api::info);
        server.at("/log").get(Api::log);
        server.at("/probe").post(Api::probe);
        server.at("/local_install").post(Api::local_install);
        server.at("/remote_install").post(Api::remote_install);
        server.at("/update/download/abort").post(Api::download_abort);
        server
    }

    async fn info(req: tide::Request<machine::Addr>) -> tide::Result<tide::Response> {
        debug!("receiving info request");
        let res = req.state().request_info().await?;
        Ok(tide::Response::builder(tide::StatusCode::Ok).body(tide::Body::from_json(&res)?).build())
    }

    async fn probe(mut req: tide::Request<machine::Addr>) -> tide::Result<tide::Response> {
        debug!("receiving probe request");
        let body = req.take_body();
        let server_address = match body.is_empty() {
            Some(true) => None,
            _ => Some(body.into_json::<api::probe::Request>().await?.custom_server),
        };
        Ok(tide::Response::try_from(req.state().request_probe(server_address).await?)?)
    }

    async fn local_install(mut req: tide::Request<machine::Addr>) -> tide::Result<tide::Response> {
        debug!("receiving local_install request");
        let file = req.body_json::<api::local_install::Request>().await?.file;
        Ok(tide::Response::try_from(req.state().request_local_install(file).await?)?)
    }

    async fn remote_install(mut req: tide::Request<machine::Addr>) -> tide::Result<tide::Response> {
        debug!("receiving remote_install request");
        let url = req.body_json::<api::remote_install::Request>().await?.url;
        Ok(tide::Response::try_from(req.state().request_remote_install(url).await?)?)
    }

    async fn log(_: tide::Request<machine::Addr>) -> tide::Result<tide::Response> {
        Ok(tide::Response::builder(tide::StatusCode::Ok)
            .body(tide::Body::from_json(&crate::logger::buffer())?)
            .build())
    }

    async fn download_abort(req: tide::Request<machine::Addr>) -> tide::Result<tide::Response> {
        debug!("receiving abort download request");
        Ok(tide::Response::try_from(req.state().request_abort_download().await?)?)
    }
}

impl TryFrom<machine::AbortDownloadResponse> for tide::Response {
    type Error = tide::Error;

    fn try_from(res: machine::AbortDownloadResponse) -> tide::Result<tide::Response> {
        Ok(match res {
            machine::AbortDownloadResponse::RequestAccepted => {
                tide::Response::builder(tide::StatusCode::Ok)
                    .body(tide::Body::from_json(&api::abort_download::Response {
                        message: "request accepted, download aborted".to_owned(),
                    })?)
                    .build()
            }
            machine::AbortDownloadResponse::InvalidState => {
                tide::Response::builder(tide::StatusCode::NotAcceptable)
                    .body(tide::Body::from_json(&api::abort_download::Refused {
                        error: "there is no download to be aborted".to_owned(),
                    })?)
                    .build()
            }
        })
    }
}

impl TryFrom<machine::ProbeResponse> for tide::Response {
    type Error = tide::Error;

    fn try_from(res: machine::ProbeResponse) -> tide::Result<tide::Response> {
        Ok(match res {
            machine::ProbeResponse::Available => tide::Response::builder(tide::StatusCode::Ok)
                .body(tide::Body::from_json(&api::probe::Response::Updating)?)
                .build(),
            machine::ProbeResponse::Unavailable => tide::Response::builder(tide::StatusCode::Ok)
                .body(tide::Body::from_json(&api::probe::Response::NoUpdate)?)
                .build(),
            machine::ProbeResponse::Delayed(d) => tide::Response::builder(tide::StatusCode::Ok)
                .body(tide::Body::from_json(&api::probe::Response::TryAgain(d))?)
                .build(),
            machine::ProbeResponse::Busy(current_state) => {
                tide::Response::builder(tide::StatusCode::Ok)
                    .body(tide::Body::from_json(&current_state)?)
                    .build()
            }
        })
    }
}

impl TryFrom<machine::StateResponse> for tide::Response {
    type Error = tide::Error;

    fn try_from(res: machine::StateResponse) -> tide::Result<tide::Response> {
        Ok(match res {
            machine::StateResponse::RequestAccepted(current_state) => {
                tide::Response::builder(tide::StatusCode::Ok)
                    .body(tide::Body::from_json(&current_state)?)
                    .build()
            }
            machine::StateResponse::InvalidState(current_state) => {
                tide::Response::builder(tide::StatusCode::NotAcceptable)
                    .body(tide::Body::from_json(&current_state)?)
                    .build()
            }
        })
    }
}
