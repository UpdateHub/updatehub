// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use common::{
    create_mock_server, format_output_client_log, format_output_server, get_output_server,
    remove_carriage_newline_characters, run_client_log, run_client_probe, FakeServer, Polling,
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
    <timestamp> INFO parking state machine
    "###);

    insta::assert_snapshot!(output_server_trce, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> DEBG loading system settings from "<file>"...
    <timestamp> DEBG runtime settings file "<file>" does not exists, using default settings...
    <timestamp> TRCE starting to handle: entry_point
    <timestamp> DEBG polling is disabled
    <timestamp> TRCE starting to handle: park
    <timestamp> INFO parking state machine
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
                level: Trace,
                message: "starting to handle: entry_point",
                time: "<timestamp>",
                data: {},
            },
            Entry {
                level: Debug,
                message: "polling is disabled",
                time: "<timestamp>",
                data: {},
            },
            Entry {
                level: Trace,
                message: "starting to handle: park",
                time: "<timestamp>",
                data: {},
            },
            Entry {
                level: Info,
                message: "parking state machine",
                time: "<timestamp>",
                data: {},
            },
        ],
    )
    "###);
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
    <timestamp> INFO probing server as we are in time
    <timestamp> INFO no update is current available for this device
    "###);

    insta::assert_snapshot!(output_server_trce, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> DEBG loading system settings from "<file>"...
    <timestamp> DEBG runtime settings file "<file>" does not exists, using default settings...
    <timestamp> TRCE starting to handle: entry_point
    <timestamp> DEBG polling is enabled
    <timestamp> TRCE starting to handle: poll
    <timestamp> INFO probing server as we are in time
    <timestamp> TRCE starting to handle: probe
    <timestamp> INFO no update is current available for this device
    <timestamp> DEBG saving runtime settings from "<file>"...
    <timestamp> TRCE starting to handle: entry_point
    <timestamp> DEBG polling is enabled
    <timestamp> TRCE starting to handle: poll
    <timestamp> DEBG delaying 86399 seconds till next probe
    "###);

    insta::assert_snapshot!(format_output_client_log(output_log), @r###"
    Ok(
        [
            Entry {
                level: Debug,
                message: "delaying 86399 seconds till next probe",
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
fn correct_config_no_update_polling_with_probe_api() {
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
    <timestamp> INFO probing server as we are in time
    <timestamp> INFO no update is current available for this device
    "###);

    insta::assert_snapshot!(output_server_trce, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> DEBG loading system settings from "<file>"...
    <timestamp> DEBG runtime settings file "<file>" does not exists, using default settings...
    <timestamp> TRCE starting to handle: entry_point
    <timestamp> DEBG polling is enabled
    <timestamp> TRCE starting to handle: poll
    <timestamp> INFO probing server as we are in time
    <timestamp> TRCE starting to handle: probe
    <timestamp> INFO no update is current available for this device
    <timestamp> DEBG saving runtime settings from "<file>"...
    <timestamp> TRCE starting to handle: entry_point
    <timestamp> DEBG polling is enabled
    <timestamp> TRCE starting to handle: poll
    <timestamp> DEBG delaying 86399 seconds till next probe
    "###);

    insta::assert_snapshot!(format_output_server(output_server_2).0.trim(), @r###"
    <timestamp> DEBG receiving probe request
    <timestamp> TRCE received external request: Probe(None)
    <timestamp> INFO no update is current available for this device
    <timestamp> DEBG saving runtime settings from "<file>"...
    <timestamp> TRCE starting to handle: entry_point
    <timestamp> DEBG polling is enabled
    <timestamp> TRCE starting to handle: poll
    <timestamp> DEBG delaying 86399 seconds till next probe
    "###);

    insta::assert_snapshot!(remove_carriage_newline_characters(output_client), @r###"
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
                message: "delaying 86399 seconds till next probe",
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
fn correct_config_no_update_no_polling_with_probe_api() {
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
    <timestamp> INFO parking state machine
    "###);

    insta::assert_snapshot!(output_server_trce, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> DEBG loading system settings from "<file>"...
    <timestamp> DEBG runtime settings file "<file>" does not exists, using default settings...
    <timestamp> TRCE starting to handle: entry_point
    <timestamp> DEBG polling is disabled
    <timestamp> TRCE starting to handle: park
    <timestamp> INFO parking state machine
    "###);

    insta::assert_snapshot!(format_output_server(output_server_2).0.trim(), @r###"
    <timestamp> DEBG receiving probe request
    <timestamp> TRCE received external request: Probe(None)
    <timestamp> INFO no update is current available for this device
    <timestamp> DEBG saving runtime settings from "<file>"...
    <timestamp> TRCE starting to handle: entry_point
    <timestamp> DEBG polling is disabled
    <timestamp> TRCE starting to handle: park
    <timestamp> INFO parking state machine
    "###);

    insta::assert_snapshot!(remove_carriage_newline_characters(output_client), @r###"
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
                level: Trace,
                message: "starting to handle: entry_point",
                time: "<timestamp>",
                data: {},
            },
            Entry {
                level: Debug,
                message: "polling is disabled",
                time: "<timestamp>",
                data: {},
            },
            Entry {
                level: Trace,
                message: "starting to handle: park",
                time: "<timestamp>",
                data: {},
            },
            Entry {
                level: Info,
                message: "parking state machine",
                time: "<timestamp>",
                data: {},
            },
        ],
    )
    "###);

    mocks.iter().for_each(|mock| mock.assert());
}

#[test]
fn correct_config_update_no_polling_with_probe_api() {
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
    <timestamp> INFO parking state machine
    "###);

    insta::assert_snapshot!(output_server_trce_1, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> DEBG loading system settings from "<file>"...
    <timestamp> DEBG runtime settings file "<file>" does not exists, using default settings...
    <timestamp> TRCE starting to handle: entry_point
    <timestamp> DEBG polling is disabled
    <timestamp> TRCE starting to handle: park
    <timestamp> INFO parking state machine
    "###);

    insta::assert_snapshot!(output_server_info_2.trim(), @r###"
    <timestamp> INFO update received: 87effe73b80453f397cee4db3c3589a8630b220876dff8fb23447315037ff96d
    <timestamp> INFO no signature key available on device, ignoring signature validation
    <timestamp> INFO installing update: 87effe73b80453f397cee4db3c3589a8630b220876dff8fb23447315037ff96d
    <timestamp> INFO using installation set as target 1
    <timestamp> INFO swapping active installation set
    <timestamp> INFO update installed successfully
    <timestamp> INFO triggering reboot
    <timestamp> INFO parking state machine
    "###);

    insta::assert_snapshot!(output_server_trce_2.trim(), @r###"
    <timestamp> DEBG receiving probe request
    <timestamp> TRCE received external request: Probe(None)
    <timestamp> INFO update received: 87effe73b80453f397cee4db3c3589a8630b220876dff8fb23447315037ff96d
    <timestamp> DEBG saving runtime settings from "<file>"...
    <timestamp> TRCE starting to handle: validation
    <timestamp> INFO no signature key available on device, ignoring signature validation
    <timestamp> TRCE starting to handle: download
    <timestamp> TRCE the following objects are missing: [("testfile", "23c3c412177bd37b9b61bf4738b18dc1fe003811c2583a14d2d9952d8b6a75b4")]
    <timestamp> DEBG starting download of: testfile (23c3c412177bd37b9b61bf4738b18dc1fe003811c2583a14d2d9952d8b6a75b4)
    <timestamp> DEBG <percentage>% of the file has been downloaded
    <timestamp> DEBG <percentage>% of the file has been downloaded
    <timestamp> DEBG <percentage>% of the file has been downloaded
    <timestamp> DEBG <percentage>% of the file has been downloaded
    <timestamp> DEBG <percentage>% of the file has been downloaded
    <timestamp> DEBG 100% of the file has been downloaded
    <timestamp> TRCE starting to handle: install
    <timestamp> INFO installing update: 87effe73b80453f397cee4db3c3589a8630b220876dff8fb23447315037ff96d
    <timestamp> INFO using installation set as target 1
    <timestamp> DEBG saving runtime settings from "<file>"...
    <timestamp> DEBG saving runtime settings from "<file>"...
    <timestamp> INFO swapping active installation set
    <timestamp> INFO update installed successfully
    <timestamp> TRCE starting to handle: reboot
    <timestamp> INFO triggering reboot
    <timestamp> TRCE starting to handle: entry_point
    <timestamp> DEBG polling is disabled
    <timestamp> TRCE starting to handle: park
    <timestamp> INFO parking state machine
    "###);

    insta::assert_snapshot!(remove_carriage_newline_characters(output_client), @r###"
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
                level: Trace,
                message: "starting to handle: entry_point",
                time: "<timestamp>",
                data: {},
            },
            Entry {
                level: Debug,
                message: "polling is disabled",
                time: "<timestamp>",
                data: {},
            },
            Entry {
                level: Trace,
                message: "starting to handle: park",
                time: "<timestamp>",
                data: {},
            },
            Entry {
                level: Info,
                message: "parking state machine",
                time: "<timestamp>",
                data: {},
            },
        ],
    )
    "###);
    mocks.iter().for_each(|mock| mock.assert());
}
