// Copyright (C) 2018, 2019, 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use updatehub_sdk::{Result, listener};

async fn download_callback(mut handler: listener::Handler) -> Result<()> {
    println!("function called when starting the Download state; it will cancel the transition");
    handler.cancel().await
}

#[tokio::main]
async fn main() -> Result<()> {
    let mut listener = listener::StateChange::default();

    // A function callback which cancels the state transition
    listener.on_state(listener::State::Download, download_callback);

    // A closure callback which prints
    listener.on_state(listener::State::Install, |handler| async move {
        println!("closure called when starting the Install state");
        handler.proceed().await
    });

    listener.listen().await
}
