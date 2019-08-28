// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use actix::Addr;
use futures::future::Future;
use slog_scope::{debug, error, info};
use std::{sync::mpsc, thread};

/// [Controller] is used to [start](Controller::start),
/// [stop](Controller::stop) and [restart](Controller::restart) the stepper
/// thread (see [start](Controller::start)).
#[derive(Debug, Default)]
pub(crate) struct Controller {
    terminate: Option<mpsc::Sender<TerminateThread>>,
}

/// Used on channel communication between the stepper thread and the
/// [Controller](Controller) to request that the stepper to stop.
struct TerminateThread;

impl Controller {
    /// Stops the current running stepper (if any) and starts a new one for the
    /// supplied Actor.
    pub(super) fn restart<A>(&mut self, addr: Addr<A>)
    where
        A: actix::Handler<super::Step>,
        A::Context: actix::dev::ToEnvelope<A, super::Step>,
    {
        self.stop();
        self.start(addr);
    }

    /// Stops the stepper if it's currently running.
    pub(super) fn stop(&mut self) {
        if let Some(sndr) = self.terminate.take() {
            // send mpsc::Sender::send Err means the channel is closed and thus the
            // thread has stopped already
            let _ = sndr.send(TerminateThread);
        }
    }

    /// Starts the stepper for the supplied Actor's address.
    ///
    /// The stepper is a thread that sends [super::Step] messages
    /// to the  actor until the step is replayed with a
    /// [super::StepTransition::Never], or a [TerminateThread] message
    /// is received.
    pub(super) fn start<A>(&mut self, addr: Addr<A>)
    where
        A: actix::Handler<super::Step>,
        A::Context: actix::dev::ToEnvelope<A, super::Step>,
    {
        let (sndr, recv) = mpsc::channel();
        self.terminate = Some(sndr);

        // We ignore errors raised by the stepper
        let _ = thread::Builder::new()
            .name(String::from("Actor Stepper"))
            .spawn(move || {
                while recv.try_recv().is_err() {
                    match addr.send(super::Step).wait() {
                        Err(e) => {
                            error!("Communication to actor failed: {:?}", e);
                        }
                        Ok(super::StepTransition::Immediate) => {}
                        Ok(super::StepTransition::Delayed(t)) => {
                            debug!("Sleeping stepper thread for: {} seconds", t.as_secs());
                            std::thread::sleep(t);
                        }
                        Ok(super::StepTransition::Never) => {
                            info!("Stopping step messages");
                            break;
                        }
                    }
                }
            });
    }
}

impl Drop for Controller {
    fn drop(&mut self) {
        self.stop();
    }
}
