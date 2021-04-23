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
    Daemon(DaemonOptions),
}

#[derive(FromArgs)]
/// Query or send control commands to the UpdateHub Agent daemon
#[argh(subcommand, name = "client")]
struct ClientOptions {
    #[argh(subcommand)]
    commands: ClientCommands,

    /// address where the UpdateHub Agent daemon is running
    #[argh(option, default = "String::from(\"localhost:8080\")")]
    daemon_address: String,

    /// set the output format to JSON
    #[argh(switch)]
    json_output: bool,
}

#[derive(FromArgs)]
#[argh(subcommand)]
enum ClientCommands {
    Info(Info),
    Log(Log),
    Probe(Probe),
    AbortDownload(AbortDownload),
    InstallPackage(InstallPackage),
}

#[derive(FromArgs)]
/// Query the current state of the UpdateHub Agent
#[argh(subcommand, name = "info")]
struct Info {}

#[derive(FromArgs)]
/// Show the UpdateHub Agent last update/probe log
#[argh(subcommand, name = "log")]
struct Log {
    /// watch for log changes
    #[argh(switch, short = 'w')]
    watch: bool,
}

#[derive(FromArgs)]
/// Probe the UpdateHub server if there is an update available
#[argh(subcommand, name = "probe")]
struct Probe {
    /// override the UpdateHub daemon to use for inquiry.
    #[argh(option)]
    server: Option<String>,
}

#[derive(FromArgs)]
/// Ask UpdateHub Agent to abort any currently running download
#[argh(subcommand, name = "abort-download")]
struct AbortDownload {}

#[derive(FromArgs)]
/// Install a package from a direct URL or a local path
#[argh(subcommand, name = "install-package")]
struct InstallPackage {
    /// the URL or path to the update package
    #[argh(positional)]
    arg: String,
}

#[derive(FromArgs)]
/// Starts the UpdateHub Agent daemon
#[argh(subcommand, name = "daemon")]
struct DaemonOptions {
    /// increase the log level verboseness
    #[argh(option, short = 'v', from_str_fn(verbosity_level), default = "slog::Level::Info")]
    verbosity: slog::Level,

    /// configuration file to use (defaults to "/etc/updatehub.conf")
    #[argh(option, short = 'c', default = "PathBuf::from(\"/etc/updatehub.conf\")")]
    config: PathBuf,
}

fn verbosity_level(value: &str) -> Result<slog::Level, String> {
    use std::str::FromStr;
    slog::Level::from_str(value).map_err(|_| format!("failed to parse verbosity level: {}", value))
}

async fn daemon_main(cmd: DaemonOptions) -> updatehub::Result<()> {
    let _guard = updatehub::logger::init(cmd.verbosity);
    info!("starting UpdateHub Agent {}", updatehub::version());

    updatehub::run(&cmd.config).await?;

    Ok(())
}

async fn client_main(client_options: ClientOptions) -> updatehub::Result<()> {
    let client = sdk::Client::new(&client_options.daemon_address);

    match client_options.commands {
        ClientCommands::Info(_) => {
            let response = client.info().await?;

            if client_options.json_output {
                println!("{}", serde_json::to_string(&response)?);
            } else {
                println!("{:#?}", response);
            }
        }
        ClientCommands::Log(log_opts) => {
            let response = client.log().await?;

            if client_options.json_output {
                println!("{}", serde_json::to_string(&response)?);
            } else if log_opts.watch {
                let mut current = 0;
                let mut last = response;
                loop {
                    for entry in last.entries.iter().skip(current) {
                        println!("{}", entry);
                    }
                    current = last.entries.len();

                    async_std::task::sleep(std::time::Duration::from_secs(1)).await;

                    let new = client.log().await?;
                    if new.entries.first() != last.entries.first() {
                        current = 0;
                    }
                    last = new;
                }
            } else {
                println!("{}", response);
            }
        }
        ClientCommands::Probe(Probe { server }) => {
            let response = client.probe(server).await?;

            if client_options.json_output {
                println!("{}", serde_json::to_string(&response)?);
            } else {
                match response {
                    sdk::api::probe::Response::Updating => {
                        println!("Update available. The update is running in background.")
                    }
                    sdk::api::probe::Response::NoUpdate => {
                        println!("There are no updates available.")
                    }
                    sdk::api::probe::Response::TryAgain(t) => {
                        println!("Server replied asking us to try again in {} seconds", t);
                    }
                }
            }
        }
        ClientCommands::AbortDownload(_) => {
            let response = client.abort_download().await?;

            if client_options.json_output {
                println!("{}", serde_json::to_string(&response)?);
            } else {
                println!("{:#?}", response);
            }
        }
        ClientCommands::InstallPackage(InstallPackage { arg }) => {
            let is_remote_install = arg.starts_with("http://") || arg.starts_with("https://");

            let response = if is_remote_install {
                client.remote_install(&arg).await?
            } else {
                let file = PathBuf::from(&arg);
                let file = if file.is_absolute() {
                    file
                } else {
                    std::env::current_dir().unwrap().join(arg)
                };

                client.local_install(&file).await?
            };

            if client_options.json_output {
                println!("{}", serde_json::to_string(&response)?);
            } else {
                println!("Local install request accepted from {:?} state", response);
                println!("Run 'updatehub client log --watch' to follow the log's progress");
            }
        }
    }

    Ok(())
}

#[async_std::main]
async fn main() {
    let cmd: TopLevel = argh::from_env();

    let res = match cmd.entry_point {
        EntryPoints::Client(client) => client_main(client).await,
        EntryPoints::Daemon(cmd) => daemon_main(cmd).await,
    };

    if let Err(e) = res {
        eprintln!("{}", e);
        std::process::exit(1);
    }
}
