// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::*;
use crate::{
    client::tests::{create_mock_server, FakeServer},
    firmware::{
        tests::{create_fake_metadata, FakeDevice},
        Metadata,
    },
    runtime_settings::RuntimeSettings,
    settings::Settings,
};
use actix::{Actor, Addr, Arbiter, System};
use futures::future::{self, Future};
use pretty_assertions::assert_eq;
use std::fs;

enum Setup {
    HasUpdate,
    NoUpdate,
}

fn setup_actor(kind: Setup) -> (Addr<Machine>, mockito::Mock, Settings, Metadata) {
    let tmpfile = tempfile::NamedTempFile::new().unwrap();
    let tmpfile = tmpfile.path();
    fs::remove_file(&tmpfile).unwrap();
    let settings = Settings::default();
    let runtime_settings = RuntimeSettings::new()
        .load(tmpfile.to_str().unwrap())
        .unwrap();
    let firmware = Metadata::from_path(&create_fake_metadata(match kind {
        Setup::HasUpdate => FakeDevice::HasUpdate,
        Setup::NoUpdate => FakeDevice::NoUpdate,
    }))
    .unwrap();
    let settings_clone = settings.clone();
    let firmware_clone = firmware.clone();
    let mock = create_mock_server(match kind {
        Setup::HasUpdate => FakeServer::HasUpdate,
        Setup::NoUpdate => FakeServer::NoUpdate,
    });
    set_shared_state!(settings, runtime_settings, firmware);

    (
        Machine(Some(StateMachine::Idle(State(Idle {})))).start(),
        mock,
        settings_clone,
        firmware_clone,
    )
}

#[test]
fn info_request() {
    let system = System::new("test");

    let (addr, _, settings, firmware) = setup_actor(Setup::NoUpdate);
    Arbiter::spawn(
        addr.send(info::Request)
            .map(move |response| {
                assert_eq!(response.state, "idle");
                assert_eq!(response.version, crate::version().to_string());
                assert_eq!(response.config, settings);
                assert_eq!(response.firmware, firmware);
            })
            .then(|_| {
                System::current().stop();
                future::ok(())
            }),
    );

    system.run().unwrap();
}

#[test]
fn step_sequence() {
    let system = System::new("test");

    let (addr, mock, ..) = setup_actor(Setup::NoUpdate);
    Arbiter::spawn(
        addr.send(info::Request)
            .map(move |response| {
                assert_eq!(response.state, "idle");
                addr
            })
            .and_then(|addr| {
                let f1 = addr.send(Step);
                let f2 = addr
                    .send(info::Request)
                    .map(|res| assert_eq!(res.state, "poll"));
                f1.then(|_| f2).then(|_| future::ok(addr))
            })
            .and_then(|addr| {
                let f1 = addr.send(Step);
                let f2 = addr
                    .send(info::Request)
                    .map(|res| assert_eq!(res.state, "probe"));
                f1.then(|_| f2).then(|_| future::ok(addr))
            })
            .and_then(|addr| {
                let f1 = addr.send(Step);
                let f2 = addr
                    .send(info::Request)
                    .map(|res| assert_eq!(res.state, "idle"));
                f1.then(|_| f2).then(|_| future::ok(addr))
            })
            .then(move |_| {
                mock.assert();
                System::current().stop();
                future::ok(())
            }),
    );

    system.run().unwrap();
}

#[test]
fn download_abort() {
    let system = System::new("test");

    let (addr, mock, ..) = setup_actor(Setup::HasUpdate);
    Arbiter::spawn(
        future::ok::<_, failure::Error>(addr)
            .and_then(|addr| {
                let f1 = addr.send(Step);
                let f2 = addr.send(Step);
                let f3 = addr.send(Step);
                let f4 = addr
                    .send(info::Request)
                    .map(|res| assert_eq!(res.state, "prepare_download"));
                f1.then(|_| f2)
                    .then(|_| f3)
                    .then(|_| f4)
                    .then(|_| future::ok(addr))
            })
            .and_then(|addr| {
                let f1 = addr.send(download_abort::Request);
                let f2 = addr
                    .send(info::Request)
                    .map(|res| assert_eq!(res.state, "idle"));
                f1.then(|_| f2).then(|_| future::ok(addr))
            })
            .then(move |_| {
                mock.assert();
                System::current().stop();
                future::ok(())
            }),
    );

    system.run().unwrap();
}

#[test]
fn trigger_probe() {
    let system = System::new("test");

    let (addr, ..) = setup_actor(Setup::NoUpdate);
    shared_state_mut!().settings.polling.enabled = false;
    Arbiter::spawn(
        future::ok::<_, failure::Error>(addr)
            .and_then(|addr| {
                let f1 = addr.send(Step);
                let f2 = addr
                    .send(info::Request)
                    .map(|res| assert_eq!(res.state, "park"));
                f1.then(|_| f2).then(|_| future::ok(addr))
            })
            .and_then(|addr| {
                let f1 = addr.send(probe::Request(None));
                let f2 = addr
                    .send(info::Request)
                    .map(|res| assert_eq!(res.state, "probe"));
                f1.then(|_| f2).then(|_| future::ok(addr))
            })
            .then(move |_| {
                System::current().stop();
                future::ok(())
            }),
    );

    system.run().unwrap();
}
