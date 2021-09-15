// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::states::machine;
use sdk::api;
use slog_scope::debug;
use warp::Filter;

type Result<T> = std::result::Result<T, warp::Rejection>;

pub(crate) struct Api(machine::Addr);

impl Api {
    pub(crate) fn server(
        addr: machine::Addr,
    ) -> warp::Server<warp::filters::BoxedFilter<(impl warp::Reply,)>> {
        let state = warp::any().map(move || addr.clone());

        let info = warp::get().and(warp::path("info")).and(state.clone()).and_then(Api::info);
        let log = warp::get().and(warp::path("log")).and_then(Api::log);
        let probe = warp::post()
            .and(warp::path("probe"))
            .and(
                warp::body::json()
                    .map(Some)
                    .or_else(|_| async { Ok::<(Option<_>,), std::convert::Infallible>((None,)) }),
            )
            .and(state.clone())
            .and_then(Api::probe);
        let local_install = warp::post()
            .and(warp::path("local_install"))
            .and(warp::body::json())
            .and(state.clone())
            .and_then(Api::local_install);
        let remote_install = warp::post()
            .and(warp::path("remote_install"))
            .and(warp::body::json())
            .and(state.clone())
            .and_then(Api::remote_install);
        let download_abort = warp::post()
            .and(warp::path!("update" / "download" / "abort"))
            .and(state)
            .and_then(Api::download_abort);

        let main_filter = warp::any()
            .and(info.or(log).or(probe).or(local_install).or(remote_install).or(download_abort))
            .boxed();
        warp::serve(main_filter)
    }

    async fn info(addr: machine::Addr) -> Result<warp::reply::Json> {
        debug!("receiving info request");
        let res = addr.request_info().await?;
        Ok(warp::reply::json(&res))
    }

    async fn log() -> Result<warp::reply::Json> {
        Ok(warp::reply::json(&crate::logger::buffer()))
    }

    async fn probe(
        req: Option<api::probe::Request>,
        addr: machine::Addr,
    ) -> Result<machine::ProbeResponse> {
        debug!("receiving probe request");
        let server_address = req.map(|b| b.custom_server);
        Ok(addr.request_probe(server_address).await?)
    }

    async fn local_install(
        req: api::local_install::Request,
        addr: machine::Addr,
    ) -> Result<machine::StateResponse> {
        debug!("receiving local_install request");
        Ok(addr.request_local_install(req.file).await?)
    }

    async fn remote_install(
        req: api::remote_install::Request,
        addr: machine::Addr,
    ) -> Result<machine::StateResponse> {
        debug!("receiving remote_install request");
        Ok(addr.request_remote_install(req.url).await?)
    }

    async fn download_abort(addr: machine::Addr) -> Result<machine::AbortDownloadResponse> {
        debug!("receiving abort download request");
        Ok(addr.request_abort_download().await?)
    }
}

impl warp::reject::Reject for crate::states::TransitionError {}

impl warp::reply::Reply for machine::AbortDownloadResponse {
    fn into_response(self) -> warp::reply::Response {
        match self {
            machine::AbortDownloadResponse::RequestAccepted => warp::reply::Response::new(
                serde_json::to_vec(&api::abort_download::Response {
                    message: "request accepted, download aborted".to_owned(),
                })
                .unwrap()
                .into(),
            ),
            machine::AbortDownloadResponse::InvalidState => warp::reply::with_status(
                warp::reply::Response::new(
                    serde_json::to_vec(&api::abort_download::Refused {
                        error: "there is no download to be aborted".to_owned(),
                    })
                    .unwrap()
                    .into(),
                ),
                warp::http::StatusCode::NOT_ACCEPTABLE,
            )
            .into_response(),
        }
    }
}

impl warp::reply::Reply for machine::ProbeResponse {
    fn into_response(self) -> warp::reply::Response {
        match self {
            machine::ProbeResponse::Available => warp::reply::Response::new(
                serde_json::to_vec(&api::probe::Response::Updating).unwrap().into(),
            ),
            machine::ProbeResponse::Unavailable => warp::reply::Response::new(
                serde_json::to_vec(&api::probe::Response::NoUpdate).unwrap().into(),
            ),
            machine::ProbeResponse::Delayed(d) => warp::reply::Response::new(
                serde_json::to_vec(&api::probe::Response::TryAgain(d)).unwrap().into(),
            ),
            machine::ProbeResponse::Busy(current_state) => {
                warp::reply::Response::new(serde_json::to_vec(&current_state).unwrap().into())
            }
        }
    }
}

impl warp::reply::Reply for machine::StateResponse {
    fn into_response(self) -> warp::reply::Response {
        match self {
            machine::StateResponse::RequestAccepted(current_state) => {
                warp::reply::Response::new(serde_json::to_vec(&current_state).unwrap().into())
            }
            machine::StateResponse::InvalidState(current_state) => warp::reply::with_status(
                warp::reply::Response::new(serde_json::to_vec(&current_state).unwrap().into()),
                warp::http::StatusCode::NOT_ACCEPTABLE,
            )
            .into_response(),
        }
    }
}
