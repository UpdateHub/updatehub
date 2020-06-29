// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::*;
use crate::cloud_mock;
use actix::{Addr, MessageResult, System};
use pretty_assertions::assert_eq;

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

fn setup_actor(kind: Setup, probe: Probe) -> (Addr<Machine>, crate::tests::TestEnvironment) {
    let setup_builder = crate::tests::TestEnvironment::build();
    let setup_builder = match probe {
        Probe::Enabled => setup_builder,
        Probe::Disabled => setup_builder.disable_polling(),
    };
    match kind {
        Setup::NoUpdate => cloud_mock::setup_fake_response(cloud_mock::FakeResponse::NoUpdate),
        Setup::HasUpdate => cloud_mock::setup_fake_response(cloud_mock::FakeResponse::HasUpdate),
    };
    let setup = setup_builder.finish();

    (
        // We use the actix::Actor::start here instead of the Machine::start in order to not
        // start the stepper and thus have control of how many steps are been sent to the Machine
        actix::Actor::start(Machine::new(
            StateMachine::EntryPoint(State(EntryPoint {})),
            setup.settings.data.clone(),
            setup.runtime_settings.data.clone(),
            setup.firmware.data.clone(),
        )),
        setup,
    )
}

#[actix_rt::test]
async fn info_request() {
    let (addr, setup) = setup_actor(Setup::NoUpdate, Probe::Enabled);
    let response = addr.send(info::Request).await.unwrap();
    assert_eq!(response.state, "entry_point");
    assert_eq!(response.version, crate::version().to_string());
    assert_eq!(response.config, setup.settings.data.0);
    assert_eq!(response.firmware, setup.firmware.data.0);
}

#[actix_rt::test]
async fn step_sequence() {
    let (addr, ..) = setup_actor(Setup::NoUpdate, Probe::Enabled);
    let response = addr.send(info::Request).await.unwrap();
    assert_eq!(response.state, "entry_point");

    addr.send(Step).await.unwrap();
    let res = addr.send(info::Request).await.unwrap();
    assert_eq!(res.state, "poll");

    addr.send(Step).await.unwrap();
    let res = addr.send(info::Request).await.unwrap();
    assert_eq!(res.state, "probe");

    addr.send(Step).await.unwrap();
    let res = addr.send(info::Request).await.unwrap();
    assert_eq!(res.state, "entry_point");
}

#[actix_rt::test]
async fn download_abort() {
    let (addr, ..) = setup_actor(Setup::HasUpdate, Probe::Enabled);
    addr.send(Step).await.unwrap(); // Idle -> Poll
    addr.send(Step).await.unwrap(); // Poll -> Probe
    addr.send(Step).await.unwrap(); // Probe -> Validation
    addr.send(Step).await.unwrap(); // Validation -> PrepareDownload
    let res = addr.send(info::Request).await.unwrap();
    assert_eq!(res.state, "prepare_download");

    addr.send(download_abort::Request).await.unwrap();
    let res = addr.send(info::Request).await.unwrap();
    assert_eq!(res.state, "entry_point");
}

#[actix_rt::test]
async fn trigger_probe() {
    let (addr, ..) = setup_actor(Setup::NoUpdate, Probe::Disabled);
    addr.send(Step).await.unwrap();
    let res = addr.send(info::Request).await.unwrap();
    assert_eq!(res.state, "park");

    addr.send(probe::Request(None)).await.unwrap().unwrap();
    let res = addr.send(info::Request).await.unwrap();
    assert_eq!(res.state, "entry_point");
}

#[actix_rt::test]
async fn local_install_probe() {
    let (addr, ..) = setup_actor(Setup::NoUpdate, Probe::Disabled);
    addr.send(Step).await.unwrap();
    let res = addr.send(info::Request).await.unwrap();
    assert_eq!(res.state, "park");

    addr.send(local_install::Request(std::path::PathBuf::from("/foo/bar"))).await.unwrap();
    let res = addr.send(info::Request).await.unwrap();
    assert_eq!(res.state, "prepare_local_install");
}

#[test]
fn stepper_with_never() {
    let sys = actix_rt::System::new("stepper_with_never");
    let mock = actix::Actor::start(FakeMachine { step_expect: 15, ..FakeMachine::default() });
    let mut stepper = super::stepper::Controller::default();
    stepper.start(mock);
    sys.run().unwrap();
}
