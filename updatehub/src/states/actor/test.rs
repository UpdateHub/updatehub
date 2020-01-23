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
use actix::{Addr, MessageResult, System};
use pretty_assertions::assert_eq;
use std::fs;

enum Setup {
    HasUpdate,
    NoUpdate,
}

enum Probe {
    Enabled,
    Disabled,
}

#[derive(Default)]
struct FakeMachine {
    step_count: usize,
    step_expect: usize,
}

impl Actor for FakeMachine {
    type Context = Context<Self>;

    // In tests, only one reference to the Actor's Addr is held, and it is held by
    // the stepper, when it stops the system can be shutdown and we can assert the
    // number of steppers received
    fn stopped(&mut self, _: &mut Context<Self>) {
        assert_eq!(self.step_count, self.step_expect);
        System::current().stop();
    }
}

impl Handler<Step> for FakeMachine {
    type Result = MessageResult<Step>;

    fn handle(&mut self, _: Step, _: &mut Context<Self>) -> Self::Result {
        self.step_count += 1;
        if self.step_count >= self.step_expect {
            MessageResult(super::StepTransition::Never)
        } else {
            MessageResult(super::StepTransition::Immediate)
        }
    }
}

fn setup_actor(kind: Setup, probe: Probe) -> (Addr<Machine>, mockito::Mock, Settings, Metadata) {
    let tmpfile = tempfile::NamedTempFile::new().unwrap();
    let tmpfile = tmpfile.path();
    fs::remove_file(&tmpfile).unwrap();
    let mut settings = Settings::default();
    settings.polling.enabled = match probe {
        Probe::Enabled => true,
        Probe::Disabled => false,
    };
    let runtime_settings = RuntimeSettings::new().load(tmpfile.to_str().unwrap()).unwrap();
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

    (
        // We use the actix::Actor::start here instead of the Machine::start in order to not
        // start the stepper and thus have control of how many steps are been sent to the Machine
        actix::Actor::start(Machine::new(
            StateMachine::Idle(State(Idle {})),
            settings,
            runtime_settings,
            firmware,
        )),
        mock,
        settings_clone,
        firmware_clone,
    )
}

#[actix_rt::test]
async fn info_request() {
    let (addr, _, settings, firmware) = setup_actor(Setup::NoUpdate, Probe::Enabled);
    let response = addr.send(info::Request).await.unwrap();
    assert_eq!(response.state, "idle");
    assert_eq!(response.version, crate::version().to_string());
    assert_eq!(response.config, settings);
    assert_eq!(response.firmware, firmware);
}

#[actix_rt::test]
async fn step_sequence() {
    let (addr, mock, ..) = setup_actor(Setup::NoUpdate, Probe::Enabled);
    let response = addr.send(info::Request).await.unwrap();
    assert_eq!(response.state, "idle");

    addr.send(Step).await.unwrap();
    let res = addr.send(info::Request).await.unwrap();
    assert_eq!(res.state, "poll");

    addr.send(Step).await.unwrap();
    let res = addr.send(info::Request).await.unwrap();
    assert_eq!(res.state, "probe");

    addr.send(Step).await.unwrap();
    let res = addr.send(info::Request).await.unwrap();
    assert_eq!(res.state, "idle");

    mock.assert();
}

#[actix_rt::test]
async fn download_abort() {
    let (addr, mock, ..) = setup_actor(Setup::HasUpdate, Probe::Enabled);
    addr.send(Step).await.unwrap();
    addr.send(Step).await.unwrap();
    addr.send(Step).await.unwrap();
    let res = addr.send(info::Request).await.unwrap();
    assert_eq!(res.state, "prepare_download");

    addr.send(download_abort::Request).await.unwrap();
    let res = addr.send(info::Request).await.unwrap();
    assert_eq!(res.state, "idle");

    mock.assert();
}

#[actix_rt::test]
async fn trigger_probe() {
    let (addr, ..) = setup_actor(Setup::NoUpdate, Probe::Disabled);
    addr.send(Step).await.unwrap();
    let res = addr.send(info::Request).await.unwrap();
    assert_eq!(res.state, "park");

    addr.send(probe::Request(None)).await.unwrap();
    let res = addr.send(info::Request).await.unwrap();
    assert_eq!(res.state, "probe");
}

#[actix_rt::test]
async fn stepper_with_never() {
    let mock = actix::Actor::start(FakeMachine { step_expect: 15, ..FakeMachine::default() });
    let mut stepper = super::stepper::Controller::default();
    stepper.start(mock);
}
