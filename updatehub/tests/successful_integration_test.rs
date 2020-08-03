// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use common::{
    create_mock_server, format_output_client_log, format_output_server, get_output_server,
    remove_carriage_newline_caracters, run_client_log, run_client_probe, FakeServer, Polling,
    Server, Settings, StopMessage,
};

pub mod common;

#[test]
fn correct_config_no_update_no_polling() {
    let setup = Settings::default();

    let (mut session, _setup) = setup.timeout(300).init_server();
    let output_server = get_output_server(&mut session, StopMessage::Polling(Polling::Disable));
    let output_log = run_client_log();

    let (output_server_trce, output_server_info) = format_output_server(output_server);

    insta::assert_snapshot!(output_server_info, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    "###);

    insta::assert_snapshot!(output_server_trce, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> DEBG loading system settings from "<file>"...
    <timestamp> DEBG runtime settings file "<file>" does not exists, using default settings...
    <timestamp> DEBG polling is disabled, parking the state machine.
    <timestamp> DEBG staying on Park state.
    "###);

    insta::assert_snapshot!(format_output_client_log(output_log), @r###"
    Ok(
        [
            Entry {
                level: Debug,
                message: "loading system settings from "<file>",
                time: "<timestamp>",
                data: {},
            },
            Entry {
                level: Debug,
                message: "runtime settings file "<file>",
                time: "<timestamp>",
                data: {},
            },
            Entry {
                level: Debug,
                message: "polling is disabled, parking the state machine.",
                time: "<timestamp>",
                data: {},
            },
            Entry {
                level: Debug,
                message: "staying on Park state.",
                time: "<timestamp>",
                data: {},
            },
        ],
    )"###);
}

#[test]
fn correct_config_no_update_polling() {
    let setup = Settings::default();
    let mocks = create_mock_server(FakeServer::NoUpdate);

    let (mut session, _setup) = setup.timeout(300).polling().init_server();
    let output_server = get_output_server(&mut session, StopMessage::Polling(Polling::Enable));
    let output_log = run_client_log();

    let (output_server_trce, output_server_info) = format_output_server(output_server);

    insta::assert_snapshot!(output_server_info, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> INFO forcing to Probe state as we are in time
    "###);

    insta::assert_snapshot!(output_server_trce, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> DEBG loading system settings from "<file>"...
    <timestamp> DEBG runtime settings file "<file>" does not exists, using default settings...
    <timestamp> DEBG polling is enabled, moving to Poll state.
    <timestamp> INFO forcing to Probe state as we are in time
    <timestamp> DEBG moving to EntryPoint state as no update is available.
    <timestamp> DEBG saving runtime settings from "<file>"...
    <timestamp> DEBG polling is enabled, moving to Poll state.
    <timestamp> DEBG moving to Probe state after delay.
    "###);

    insta::assert_snapshot!(format_output_client_log(output_log), @r###"
    Ok(
        [
            Entry {
                level: Debug,
                message: "moving to Probe state after delay.",
                time: "<timestamp>",
                data: {},
            },
            Entry {
                level: Trace,
                message: "delaying transition for: 86399 seconds",
                time: "<timestamp>",
                data: {},
            },
            Entry {
                level: Debug,
                message: "receiving log request",
                time: "<timestamp>",
                data: {},
            },
        ],
    )
    "###);

    mocks.iter().for_each(|mock| mock.assert());
}

#[test]
fn correct_config_no_update_polling_probe_api() {
    let setup = Settings::default();
    let mocks = create_mock_server(FakeServer::NoUpdate);

    let (mut session, _setup) = setup.timeout(300).polling().init_server();
    let output_server_1 = get_output_server(&mut session, StopMessage::Polling(Polling::Enable));

    mocks.iter().for_each(|mock| mock.assert());

    let output_client = run_client_probe(Server::Standard);
    let output_server_2 = get_output_server(&mut session, StopMessage::Polling(Polling::Enable));
    let output_log = run_client_log();

    let mut iter = output_server_2.lines();
    iter.next();
    let output_server_2 = iter.fold(String::default(), |acc, l| acc + l + "\n");

    let (output_server_trce, output_server_info) = format_output_server(output_server_1);

    insta::assert_snapshot!(output_server_info, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> INFO forcing to Probe state as we are in time
    "###);

    insta::assert_snapshot!(output_server_trce, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> DEBG loading system settings from "<file>"...
    <timestamp> DEBG runtime settings file "<file>" does not exists, using default settings...
    <timestamp> DEBG polling is enabled, moving to Poll state.
    <timestamp> INFO forcing to Probe state as we are in time
    <timestamp> DEBG moving to EntryPoint state as no update is available.
    <timestamp> DEBG saving runtime settings from "<file>"...
    <timestamp> DEBG polling is enabled, moving to Poll state.
    <timestamp> DEBG moving to Probe state after delay.
    "###);

    insta::assert_snapshot!(format_output_server(output_server_2).0.trim(), @r###"
    <timestamp> DEBG receiving probe request
    <timestamp> TRCE Received external request: Probe(None)
    <timestamp> DEBG saving runtime settings from "<file>"...
    <timestamp> DEBG polling is enabled, moving to Poll state.
    <timestamp> DEBG moving to Probe state after delay.
    "###);

    insta::assert_snapshot!(remove_carriage_newline_caracters(output_client), @r###"
    Ok(
        Response {
            update_available: false,
            try_again_in: None,
        },
    )
    "###);

    insta::assert_snapshot!(format_output_client_log(output_log), @r###"
    Ok(
        [
            Entry {
                level: Debug,
                message: "moving to Probe state after delay.",
                time: "<timestamp>",
                data: {},
            },
            Entry {
                level: Trace,
                message: "delaying transition for: 86399 seconds",
                time: "<timestamp>",
                data: {},
            },
            Entry {
                level: Debug,
                message: "receiving log request",
                time: "<timestamp>",
                data: {},
            },
        ],
    )
    "###);
}

#[test]
fn correct_config_no_update_no_polling_probe_api() {
    let setup = Settings::default();
    let mocks = create_mock_server(FakeServer::NoUpdate);

    let (mut session, _setup) = setup.timeout(300).init_server();
    let output_server_1 = get_output_server(&mut session, StopMessage::Polling(Polling::Disable));

    let output_client = run_client_probe(Server::Standard);
    let output_server_2 = get_output_server(&mut session, StopMessage::Polling(Polling::Disable));
    let output_log = run_client_log();

    let (output_server_trce, output_server_info) = format_output_server(output_server_1);

    insta::assert_snapshot!(output_server_info, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    "###);

    insta::assert_snapshot!(output_server_trce, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> DEBG loading system settings from "<file>"...
    <timestamp> DEBG runtime settings file "<file>" does not exists, using default settings...
    <timestamp> DEBG polling is disabled, parking the state machine.
    <timestamp> DEBG staying on Park state.
    "###);

    insta::assert_snapshot!(format_output_server(output_server_2).0.trim(), @r###"
    <timestamp> DEBG receiving probe request
    <timestamp> TRCE Received external request: Probe(None)
    <timestamp> DEBG saving runtime settings from "<file>"...
    <timestamp> DEBG polling is disabled, parking the state machine.
    <timestamp> DEBG staying on Park state.
    "###);

    insta::assert_snapshot!(remove_carriage_newline_caracters(output_client), @r###"
    Ok(
        Response {
            update_available: false,
            try_again_in: None,
        },
    )
    "###);

    insta::assert_snapshot!(format_output_client_log(output_log), @r###"
    Ok(
        [
            Entry {
                level: Debug,
                message: "loading system settings from "<file>",
                time: "<timestamp>",
                data: {},
            },
            Entry {
                level: Debug,
                message: "runtime settings file "<file>",
                time: "<timestamp>",
                data: {},
            },
            Entry {
                level: Debug,
                message: "polling is disabled, parking the state machine.",
                time: "<timestamp>",
                data: {},
            },
            Entry {
                level: Debug,
                message: "staying on Park state.",
                time: "<timestamp>",
                data: {},
            },
        ],
    )"###);

    mocks.iter().for_each(|mock| mock.assert());
}

#[test]
fn correct_config_update_no_polling_probe_api() {
    let setup = Settings::default();

    let (mut session, setup) = setup.timeout(300).init_server();
    let mocks = create_mock_server(FakeServer::HasUpdate(setup.firmware.data.product_uid.clone()));
    let output_server_1 = get_output_server(&mut session, StopMessage::Polling(Polling::Disable));

    let output_client = run_client_probe(Server::Standard);
    let output_server_2 = get_output_server(&mut session, StopMessage::Polling(Polling::Disable));
    let output_log = run_client_log();

    let (output_server_trce_1, output_server_info_1) = format_output_server(output_server_1);
    let (output_server_trce_2, output_server_info_2) = format_output_server(output_server_2);

    insta::assert_snapshot!(output_server_info_1, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    "###);

    insta::assert_snapshot!(output_server_trce_1, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> DEBG loading system settings from "<file>"...
    <timestamp> DEBG runtime settings file "<file>" does not exists, using default settings...
    <timestamp> DEBG polling is disabled, parking the state machine.
    <timestamp> DEBG staying on Park state.
    "###);

    insta::assert_snapshot!(output_server_trce_2.trim(), @r###"
    <timestamp> DEBG receiving probe request
    <timestamp> TRCE Received external request: Probe(None)
    <timestamp> DEBG saving runtime settings from "<file>"...
    <timestamp> TRCE moving to Download state to process the update package.
    <timestamp> DEBG <percentage>% of the file has been downloaded
    <timestamp> DEBG <percentage>% of the file has been downloaded
    <timestamp> DEBG <percentage>% of the file has been downloaded
    <timestamp> DEBG <percentage>% of the file has been downloaded
    <timestamp> DEBG <percentage>% of the file has been downloaded
    <timestamp> DEBG 100% of the file has been downloaded
    <timestamp> INFO installing update: 87effe73b80453f397cee4db3c3589a8630b220876dff8fb23447315037ff96d
    <timestamp> INFO using installation set as target 1
    <timestamp> DEBG saving runtime settings from "<file>"...
    <timestamp> DEBG saving runtime settings from "<file>"...
    <timestamp> INFO swapping active installation set
    <timestamp> INFO update installed successfully
    <timestamp> INFO triggering reboot
    <timestamp> DEBG polling is disabled, parking the state machine.
    <timestamp> DEBG staying on Park state.
    "###);

    insta::assert_snapshot!(output_server_info_2.trim(), @r###"
    <timestamp> INFO installing update: 87effe73b80453f397cee4db3c3589a8630b220876dff8fb23447315037ff96d
    <timestamp> INFO using installation set as target 1
    <timestamp> INFO swapping active installation set
    <timestamp> INFO update installed successfully
    <timestamp> INFO triggering reboot
    "###);

    insta::assert_snapshot!(remove_carriage_newline_caracters(output_client), @r###"
    Ok(
        Response {
            update_available: true,
            try_again_in: None,
        },
    )
    "###);
    insta::assert_snapshot!(format_output_client_log(output_log), @r###"
    Ok(
        [
            Entry {
                level: Debug,
                message: "loading system settings from "<file>",
                time: "<timestamp>",
                data: {},
            },
            Entry {
                level: Debug,
                message: "runtime settings file "<file>",
                time: "<timestamp>",
                data: {},
            },
            Entry {
                level: Debug,
                message: "polling is disabled, parking the state machine.",
                time: "<timestamp>",
                data: {},
            },
            Entry {
                level: Debug,
                message: "staying on Park state.",
                time: "<timestamp>",
                data: {},
            },
        ],
    )"###);
    mocks.iter().for_each(|mock| mock.assert());
}
