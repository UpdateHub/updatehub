// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::states::actor;
use actix::Addr;
use actix_web::{error::Error, http, App, HttpRequest, HttpResponse, Json, Responder, Result};
use futures::future::Future;
use serde::Serialize;
use serde_json::json;

pub fn app(addr: Addr<actor::Machine>) -> App<API> {
    let agent = API::new(addr);
    App::with_state(agent)
        .route("/info", http::Method::GET, API::info)
        .route("/log", http::Method::GET, API::probe)
        .route("/probe", http::Method::POST, API::probe)
        .route(
            "/update/download/abort",
            http::Method::POST,
            API::download_abort,
        )
}

pub struct API(Addr<actor::Machine>);

impl API {
    fn new(addr: Addr<actor::Machine>) -> Self {
        Self(addr)
    }

    fn info(req: HttpRequest<API>) -> impl Responder {
        Json(req.state().0.send(actor::info::Request).wait().unwrap())
    }

    fn probe(req: HttpRequest<API>) -> impl Responder {
        let server_address = req
            .match_info()
            .get("server-address")
            .map(std::string::ToString::to_string);

        req.state()
            .0
            .send(actor::probe::Request(server_address))
            .wait()
    }

    fn log(_req: HttpRequest<API>) -> impl Responder {
        Json(crate::logger::buffer())
    }

    fn download_abort(req: HttpRequest<API>) -> impl Responder {
        req.state().0.send(actor::download_abort::Request).wait()
    }
}

impl Responder for actor::download_abort::Response {
    type Error = Error;
    type Item = HttpResponse;

    fn respond_to<S: 'static>(self, _: &HttpRequest<S>) -> Result<Self::Item, Self::Error> {
        match self {
            actor::download_abort::Response::RequestAccepted => {
                Ok(HttpResponse::Ok().json(json!({
                    "message": "request accepted, download aborted"
                })))
            }
            actor::download_abort::Response::InvalidState => {
                Ok(HttpResponse::BadRequest().json(json!({
                    "error": "there is no download to be aborted"
                })))
            }
        }
    }
}

impl Responder for actor::probe::Response {
    type Error = Error;
    type Item = HttpResponse;

    fn respond_to<S: 'static>(self, _: &HttpRequest<S>) -> Result<Self::Item, Self::Error> {
        #[derive(Serialize)]
        struct Payload {
            busy: bool,
            #[serde(rename = "current-state")]
            state: String,
        }

        match self {
            actor::probe::Response::RequestAccepted(state) => {
                Ok(HttpResponse::Ok().json(Payload { busy: false, state }))
            }
            actor::probe::Response::InvalidState(state) => {
                Ok(HttpResponse::Ok().json(Payload { busy: true, state }))
            }
        }
    }
}
