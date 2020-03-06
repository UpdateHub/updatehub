// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0
use argh::FromArgs;
use slog_scope::info;

#[derive(FromArgs)]
/// Top-level command.
struct TopLevel {
    #[argh(subcommand)]
    entry_point: EntryPoints,
}

#[derive(FromArgs)]
#[argh(subcommand)]
enum EntryPoints {
    Cli(CliOptions),
    Server(ServerOptions),
}

#[derive(FromArgs)]
/// Cli subcommand
#[argh(subcommand, name = "cli")]
struct CliOptions {
    #[argh(subcommand)]
    commands: CliCommands,
}

#[derive(FromArgs)]
#[argh(subcommand)]
enum CliCommands {
    Probe(SubCommandProbe),
    ProbeCustom(SubCommandProbeCustom),
    Info(SubCommandInfo),
    Log(SubCommandLog),
    Abort(SubCommandAbort),
    Download(SubCommandDownload),
}

#[derive(FromArgs)]
/// Probe subcommand
#[argh(subcommand, name = "probe")]
struct SubCommandProbe {}

#[derive(FromArgs)]
/// Info subcommand
#[argh(subcommand, name = "info")]
struct SubCommandInfo {}

#[derive(FromArgs)]
/// Log subcommand
#[argh(subcommand, name = "log")]
struct SubCommandLog {}

#[derive(FromArgs)]
/// Abort subcommand
#[argh(subcommand, name = "abort")]
struct SubCommandAbort {}

#[derive(FromArgs)]
/// Download subcommand
#[argh(subcommand, name = "download")]
struct SubCommandDownload {}

#[derive(FromArgs)]
/// Probe custom subcommand
#[argh(subcommand, name = "probe-custom")]
struct SubCommandProbeCustom {
    /// custom address to try probe
    #[argh(positional)]
    server: String,
}

#[derive(FromArgs)]
/// Server subcommand
#[argh(subcommand, name = "server")]
struct ServerOptions {
    /// increase the verboseness level
    #[argh(option, short = 'v', from_str_fn(verbosity_level))]
    verbose: Option<slog::Level>,
}

fn verbosity_level(value: &str) -> Result<slog::Level, String> {
    use std::str::FromStr;
    slog::Level::from_str(value).map_err(|_| format!("Failed to parse verbosity level: {}", value))
}

async fn server_main(cmd: ServerOptions) -> updatehub::Result<()> {
    updatehub::logger::init(cmd.verbose.unwrap_or(slog::Level::Info));
    info!("Starting UpdateHub Agent {}", updatehub::version());

    let settings = updatehub::Settings::load()?;
    updatehub::run(settings).await?;

    Ok(())
}

async fn cli_main(cmd: CliCommands) -> updatehub::Result<()> {
    let client = sdk::Client::new("localhost:8080");

    let res = match cmd {
        CliCommands::Info(_) => client.info().await,
        CliCommands::Probe(_) => todo!("probe"),
        CliCommands::Log(_) => todo!("log"),
        CliCommands::Abort(_) => todo!("abort"),
        CliCommands::Download(_) => todo!("Download"),
        CliCommands::ProbeCustom(input) => todo!("Probe custom {:?}", input.server),
    };

    dbg!(res).unwrap();

    Ok(())
}

#[actix_rt::main]
async fn main() {
    let cmd: TopLevel = argh::from_env();

    let res = match cmd.entry_point {
        EntryPoints::Cli(cli) => cli_main(cli.commands).await,
        EntryPoints::Server(cmd) => server_main(cmd).await,
    };

    if let Err(e) = res {
        eprintln!("{}", e);
        std::process::exit(1);
    }
}
