// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0
use argh::FromArgs;
use slog_scope::info;
use std::path::PathBuf;

#[derive(FromArgs)]
/// Top-level command.
struct TopLevel {
    #[argh(subcommand)]
    entry_point: EntryPoints,
}

#[derive(FromArgs)]
#[argh(subcommand)]
enum EntryPoints {
    Client(ClientOptions),
    Server(ServerOptions),
}

#[derive(FromArgs)]
/// Client subcommand
#[argh(subcommand, name = "client")]
struct ClientOptions {
    #[argh(subcommand)]
    commands: ClientCommands,
}

#[derive(FromArgs)]
#[argh(subcommand)]
enum ClientCommands {
    Info(Info),
    Log(Log),
    Probe(Probe),
    AbortDownload(AbortDownload),
    LocalInstall(LocalInstall),
    RemoteInstall(RemoteInstall),
}

#[derive(FromArgs)]
/// Fetches information about the current state of the agent
#[argh(subcommand, name = "info")]
struct Info {}

#[derive(FromArgs)]
/// Fetches the available log entries for the last update cycle
#[argh(subcommand, name = "log")]
struct Log {}

#[derive(FromArgs)]
/// Checks if the server has a new update for this device.
///
/// A custom server for the update cycle can be specified via the ´--server´
#[argh(subcommand, name = "probe")]
struct Probe {
    /// custom address to try probe
    #[argh(option)]
    server: Option<String>,
}

#[derive(FromArgs)]
/// Abort current running download
#[argh(subcommand, name = "abort-download")]
struct AbortDownload {}

#[derive(FromArgs)]
/// Request agent to install a local update package
#[argh(subcommand, name = "local-install")]
struct LocalInstall {
    /// path to the update package
    #[argh(positional)]
    file: PathBuf,
}

#[derive(FromArgs)]
/// Request agent to download and install a package from a direct URL
#[argh(subcommand, name = "remote-install")]
struct RemoteInstall {
    /// the URL to get the update package
    #[argh(positional)]
    url: String,
}

#[derive(FromArgs)]
/// Server subcommand
#[argh(subcommand, name = "server")]
struct ServerOptions {
    /// increase the verboseness level
    #[argh(option, short = 'v', from_str_fn(verbosity_level))]
    verbosity: Option<slog::Level>,
}

fn verbosity_level(value: &str) -> Result<slog::Level, String> {
    use std::str::FromStr;
    slog::Level::from_str(value).map_err(|_| format!("Failed to parse verbosity level: {}", value))
}

async fn server_main(cmd: ServerOptions) -> updatehub::Result<()> {
    updatehub::logger::init(cmd.verbosity.unwrap_or(slog::Level::Info));
    info!("Starting UpdateHub Agent {}", updatehub::version());

    let settings = updatehub::Settings::load()?;
    updatehub::run(settings).await?;

    Ok(())
}

async fn client_main(cmd: ClientCommands) -> updatehub::Result<()> {
    let client = sdk::Client::new("localhost:8080");

    match cmd {
        ClientCommands::Info(_) => println!("{:#?}", client.info().await),
        ClientCommands::Log(_) => println!("{:#?}", client.log().await),
        ClientCommands::Probe(Probe { server }) => println!("{:#?}", client.probe(server).await),
        ClientCommands::AbortDownload(_) => println!("{:#?}", client.abort_download().await),
        ClientCommands::LocalInstall(LocalInstall { file }) => {
            let file =
                if file.is_absolute() { file } else { std::env::current_dir().unwrap().join(file) };
            println!("{:#?}", client.local_install(&file).await)
        }
        ClientCommands::RemoteInstall(RemoteInstall { url }) => {
            println!("{:#?}", client.remote_install(&url).await)
        }
    }

    Ok(())
}

#[actix_rt::main]
async fn main() {
    let cmd: TopLevel = argh::from_env();

    let res = match cmd.entry_point {
        EntryPoints::Client(client) => client_main(client.commands).await,
        EntryPoints::Server(cmd) => server_main(cmd).await,
    };

    if let Err(e) = res {
        eprintln!("{}", e);
        std::process::exit(1);
    }
}
