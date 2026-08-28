// Copyright (C) 2026 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::*;
use crate::states::{Park, Poll};
use chrono::Utc;

/// Wakes the machine while a probe is still waiting on the server, then lets
/// the server answer.
///
/// A handler suspended on a slow server is the case that matters: the wait the
/// machine was racing resolves while a request is only half answered, and the
/// answer still has to reach the client.
async fn probe_interrupted_by_a_wake(state: State, setup: crate::tests::TestEnvironment) {
    let context = setup.gen_context();
    let waker = context.waker.sender.clone();
    let machine = StateMachine { state, context };
    let addr = machine.address();

    tokio::task::LocalSet::new()
        .run_until(async move {
            tokio::task::spawn_local(machine.start());

            // A completed round trip proves the machine reached the transition
            // under test instead of still walking the loop.
            addr.request_info().await.unwrap();

            let (release, arrived) = crate::cloud_mock::gate_probes();
            let (response, ()) = futures_util::future::join(addr.request_probe(None), async {
                arrived.recv().await.unwrap();
                waker.send(()).await.unwrap();
                release.send(()).await.unwrap();
            })
            .await;

            assert!(
                matches!(response, Ok(ProbeResponse::Unavailable)),
                "the probe was answered with {response:?}"
            );
        })
        .await;
}

#[tokio::test]
async fn a_slow_probe_is_answered_while_parked() {
    let setup = crate::tests::TestEnvironment::build().disable_polling().finish();
    probe_interrupted_by_a_wake(State::Park(Park {}), setup).await;
}

#[tokio::test]
async fn a_slow_probe_is_answered_while_delaying() {
    let mut setup = crate::tests::TestEnvironment::build().finish();
    // Polling just happened, so `Poll` delays for the whole interval instead of
    // probing right away.
    setup.runtime_settings.data.polling.last = Utc::now();
    probe_interrupted_by_a_wake(State::Poll(Poll {}), setup).await;
}

/// Answering one request must not stall the next. Each accepted request signals
/// the waker, whose single slot the loop only drains between transitions.
#[tokio::test]
async fn queued_requests_are_all_answered() {
    let setup = crate::tests::TestEnvironment::build().disable_polling().finish();
    let machine = StateMachine { state: State::Park(Park {}), context: setup.gen_context() };
    let addr = machine.address();

    tokio::task::LocalSet::new()
        .run_until(async move {
            tokio::task::spawn_local(machine.start());
            addr.request_info().await.unwrap();

            let (first, second, third) = futures_util::future::join3(
                addr.request_probe(None),
                addr.request_probe(None),
                addr.request_probe(None),
            )
            .await;

            for (which, response) in [("first", first), ("second", second), ("third", third)] {
                assert!(
                    matches!(response, Ok(ProbeResponse::Unavailable)),
                    "{which} probe was answered with {response:?}"
                );
            }
        })
        .await;
}

#[tokio::test]
async fn state_transition_survives_an_abandoned_request() {
    let setup = crate::tests::TestEnvironment::build().finish();
    let mut context = setup.gen_context();

    let (responder, reply) = async_channel::bounded(1);
    // The client gave up before the agent got to answer.
    drop(reply);

    let new_state = State::Park(Park {})
        .handle_communication(
            Message::LocalInstall(std::path::PathBuf::from("/tmp/update.uhupkg")),
            responder,
            &mut context,
        )
        .await;

    let machine = new_state.expect("the accepted local install was dropped");
    assert_state!(machine, PrepareLocalInstall);
}
