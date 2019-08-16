// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use actix::{Addr, Arbiter};
use futures::{future::Future, sync::oneshot::Canceled};
#[cfg(not(test))]
use {
    futures::Async,
    slog_scope::{debug, error, info},
};

#[derive(Default)]
pub(super) struct Controller {
    state: Option<Box<dyn Future<Item = (), Error = Canceled>>>,
    arbiter: Arbiter,
}

impl Controller {
    /// Ensures that there is a stepper running.
    /// The stepper is a thread that sends `supper::Step` messages to the
    /// Machine actor until the step is replayed with a StepTransition::Never.
    #[cfg(not(test))]
    pub(super) fn ensure_running(&mut self, addr: Addr<super::Machine>) {
        if let Some(ref mut fut) = self.state {
            // If future is still not ready the stepper is already running
            if let Ok(Async::NotReady) = fut.poll() {
                return;
            }
        }

        self.state = Some(Box::new(self.arbiter.exec(move || {
            while addr.connected() {
                match addr.send(super::Step).wait() {
                    Err(e) => error!("Communication to actor failed: {:?}", e),
                    Ok(super::StepTransition::Delayed(t)) => {
                        debug!("Sleeping stepper thread for: {} seconds", t.as_secs());
                        std::thread::sleep(t);
                    }
                    Ok(super::StepTransition::Immediate) => {}
                    Ok(super::StepTransition::Never) => {
                        info!("Stopping step messages");
                        break;
                    }
                }
            }
        })));
    }

    // On test we don't want the state machine to be progressing on it's own
    #[cfg(test)]
    pub(super) fn ensure_running(&mut self, _: Addr<super::Machine>) {}
}
