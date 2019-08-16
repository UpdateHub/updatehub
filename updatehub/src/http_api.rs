// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::states::actor;
use actix_web::{web, Error, HttpRequest, HttpResponse, Responder};
use futures::future::Future;
use serde::Serialize;
use serde_json::json;

pub(crate) struct API(actix::Addr<actor::Machine>);

impl API {
    pub(crate) fn configure(cfg: &mut web::ServiceConfig, addr: actix::Addr<actor::Machine>) {
        cfg.data(Self(addr))
            .route("/info", web::get().to(API::info))
            .route("/log", web::get().to(API::log))
            .route("/probe", web::post().to(API::probe))
            .route(
                "/update/download/abort",
                web::post().to(API::download_abort),
            );
    }

    fn info(agent: web::Data<API>) -> impl Responder {
        web::Json(agent.0.send(actor::info::Request).wait().unwrap())
    }

    fn probe(agent: web::Data<API>, server_address: Option<String>) -> impl Responder {
        agent.0.send(actor::probe::Request(server_address)).wait()
    }

    fn log() -> impl Responder {
        web::Json(crate::logger::buffer())
    }

    fn download_abort(agent: web::Data<API>) -> impl Responder {
        agent.0.send(actor::download_abort::Request).wait()
    }
}

impl Responder for actor::download_abort::Response {
    type Error = Error;
    type Future = HttpResponse;

    fn respond_to(self, _: &HttpRequest) -> Self::Future {
        match self {
            actor::download_abort::Response::RequestAccepted => HttpResponse::Ok().json(json!({
                "message": "request accepted, download aborted"
            })),
            actor::download_abort::Response::InvalidState => {
                HttpResponse::BadRequest().json(json!({
                    "error": "there is no download to be aborted"
                }))
            }
        }
    }
}

impl Responder for actor::probe::Response {
    type Error = Error;
    type Future = HttpResponse;

    fn respond_to(self, _: &HttpRequest) -> Self::Future {
        #[derive(Serialize)]
        struct Payload {
            busy: bool,
            #[serde(rename = "current-state")]
            state: String,
        }

        match self {
            actor::probe::Response::RequestAccepted(state) => {
                HttpResponse::Ok().json(Payload { busy: false, state })
            }
            actor::probe::Response::InvalidState(state) => {
                HttpResponse::Ok().json(Payload { busy: true, state })
            }
        }
    }
}
