// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::states::machine;
use actix_web::{http::StatusCode, web, HttpRequest, HttpResponse, Responder};
use sdk::api;
use slog_scope::debug;
use thiserror::Error;

pub(crate) struct API(machine::Addr);

type Result<T> = std::result::Result<T, Error>;

#[derive(Debug, Error)]
enum Error {
    #[error("State has failed to handle the request: {0}")]
    State(#[from] crate::states::TransitionError),
}

impl API {
    pub(crate) fn configure(cfg: &mut web::ServiceConfig, addr: machine::Addr) {
        cfg.data(Self(addr))
            .route("/info", web::get().to(API::info))
            .route("/log", web::get().to(API::log))
            .route("/probe", web::post().to(API::probe))
            .route("/local_install", web::post().to(API::local_install))
            .route("/remote_install", web::post().to(API::remote_install))
            .route("/update/download/abort", web::post().to(API::download_abort));
    }

    async fn info(agent: web::Data<API>) -> HttpResponse {
        debug!("receiving info request");
        HttpResponse::Ok().json(agent.0.request_info().await)
    }

    async fn probe(
        agent: web::Data<API>,
        server_address: Option<web::Json<api::probe::Request>>,
    ) -> Result<machine::ProbeResponse> {
        let server_address = server_address.map(|r| r.into_inner().custom_server);
        debug!("receiving probe request with {:?}", server_address);
        Ok(agent.0.request_probe(server_address).await?)
    }

    async fn local_install(
        agent: web::Data<API>,
        req: web::Json<api::local_install::Request>,
    ) -> machine::StateResponse {
        debug!("receiving local_install request with {:?}", req);
        agent.0.request_local_install(req.into_inner().file).await
    }

    async fn remote_install(
        agent: web::Data<API>,
        req: web::Json<api::remote_install::Request>,
    ) -> machine::StateResponse {
        debug!("receiving remote_install request with {:?}", req);
        agent.0.request_remote_install(req.into_inner().url).await
    }

    async fn log() -> HttpResponse {
        debug!("receiving log request");
        HttpResponse::Ok().json(crate::logger::buffer())
    }

    async fn download_abort(agent: web::Data<API>) -> machine::AbortDownloadResponse {
        debug!("receiving abort download request");
        agent.0.request_abort_download().await
    }
}

impl Responder for machine::AbortDownloadResponse {
    type Error = actix_web::Error;
    type Future = HttpResponse;

    fn respond_to(self, _: &HttpRequest) -> Self::Future {
        match self {
            machine::AbortDownloadResponse::RequestAccepted => {
                HttpResponse::Ok().json(api::abort_download::Response {
                    message: "request accepted, download aborted".to_owned(),
                })
            }
            machine::AbortDownloadResponse::InvalidState => {
                HttpResponse::BadRequest().json(api::abort_download::Refused {
                    error: "there is no download to be aborted".to_owned(),
                })
            }
        }
    }
}

impl Responder for machine::ProbeResponse {
    type Error = actix_web::Error;
    type Future = HttpResponse;

    fn respond_to(self, _: &HttpRequest) -> Self::Future {
        match self {
            machine::ProbeResponse::Available => HttpResponse::Ok()
                .json(api::probe::Response { update_available: true, try_again_in: None }),
            machine::ProbeResponse::Unavailable => HttpResponse::Ok()
                .json(api::probe::Response { update_available: false, try_again_in: None }),
            machine::ProbeResponse::Delayed(d) => HttpResponse::Ok()
                .json(api::probe::Response { update_available: false, try_again_in: Some(d) }),
            machine::ProbeResponse::Busy(current_state) => {
                HttpResponse::Accepted().json(api::state::Response { busy: true, current_state })
            }
        }
    }
}

impl Responder for machine::StateResponse {
    type Error = actix_web::Error;
    type Future = HttpResponse;

    fn respond_to(self, _: &HttpRequest) -> Self::Future {
        match self {
            machine::StateResponse::RequestAccepted(current_state) => {
                HttpResponse::Ok().json(api::state::Response { busy: false, current_state })
            }
            machine::StateResponse::InvalidState(current_state) => {
                HttpResponse::UnprocessableEntity()
                    .json(api::state::Response { busy: true, current_state })
            }
        }
    }
}

impl actix_web::ResponseError for Error {
    fn status_code(&self) -> StatusCode {
        StatusCode::INTERNAL_SERVER_ERROR
    }

    fn error_response(&self) -> HttpResponse {
        HttpResponse::InternalServerError().finish()
    }
}
