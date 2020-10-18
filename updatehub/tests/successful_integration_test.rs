// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use common::{
    create_mock_server, get_output_server, remove_carriage_newline_characters, run_client_log,
    run_client_probe, FakeServer, Polling, Server, Settings, StopMessage,
};

pub mod common;

#[test]
fn correct_config_no_update_no_polling() {
    let (mut session, setup) = Settings::default().timeout(300).init_server();

    let (output_server_trce, output_server_info) =
        get_output_server(&mut session, StopMessage::Polling(Polling::Disable));
    let output_log = run_client_log(&setup.settings.data.network.listen_socket);

    insta::assert_snapshot!(output_server_info, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> INFO parking state machine
    "###);

    insta::assert_snapshot!(output_server_trce, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> DEBG loading system settings from "<file>"
    <timestamp> DEBG runtime settings file "<file>" does not exists, using default settings
    <timestamp> TRCE starting to handle: entry_point
    <timestamp> DEBG polling is disabled
    <timestamp> TRCE starting to handle: park
    <timestamp> INFO parking state machine
    "###);

    insta::assert_snapshot!(output_log, @r###"
    <timestamp> DEBG loading system settings from "<file>"
    <timestamp> DEBG runtime settings file "<file>" does not exists, using default settings
    <timestamp> TRCE starting to handle: entry_point
    <timestamp> DEBG polling is disabled
    <timestamp> TRCE starting to handle: park
    <timestamp> INFO parking state machine
    "###);
}

#[test]
fn correct_config_no_update_polling() {
    let mocks = create_mock_server(FakeServer::NoUpdate);
    let (mut session, setup) = Settings::default().timeout(300).polling().init_server();

    let (output_server_trce, output_server_info) =
        get_output_server(&mut session, StopMessage::Polling(Polling::Enable));
    let output_log = run_client_log(&setup.settings.data.network.listen_socket);

    mocks.iter().for_each(|mock| mock.assert());

    insta::assert_snapshot!(output_server_info, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> INFO probing server as we are in time
    <timestamp> INFO no update is current available for this device
    "###);

    insta::assert_snapshot!(output_server_trce, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> DEBG loading system settings from "<file>"
    <timestamp> DEBG runtime settings file "<file>" does not exists, using default settings
    <timestamp> TRCE starting to handle: entry_point
    <timestamp> DEBG polling is enabled
    <timestamp> TRCE starting to handle: poll
    <timestamp> INFO probing server as we are in time
    <timestamp> TRCE starting to handle: probe
    <timestamp> INFO no update is current available for this device
    <timestamp> DEBG updating last polling time
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> TRCE starting to handle: entry_point
    <timestamp> DEBG polling is enabled
    <timestamp> TRCE starting to handle: poll
    <timestamp> DEBG delaying <time> till next probe
    "###);

    insta::assert_snapshot!(output_log, @r###"
    <timestamp> DEBG loading system settings from "<file>"
    <timestamp> DEBG runtime settings file "<file>" does not exists, using default settings
    <timestamp> TRCE starting to handle: entry_point
    <timestamp> DEBG polling is enabled
    <timestamp> TRCE starting to handle: poll
    <timestamp> INFO probing server as we are in time
    <timestamp> TRCE starting to handle: probe
    <timestamp> INFO no update is current available for this device
    <timestamp> DEBG updating last polling time
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> TRCE starting to handle: entry_point
    <timestamp> DEBG polling is enabled
    <timestamp> TRCE starting to handle: poll
    <timestamp> DEBG delaying <time> till next probe
    <timestamp> TRCE delaying transition for: <time>
    <timestamp> DEBG receiving log request
    "###);
}

#[test]
fn correct_config_no_update_polling_with_probe_api() {
    let mocks = create_mock_server(FakeServer::NoUpdate);
    let (mut session, setup) = Settings::default().timeout(300).polling().init_server();

    let (output_server_trce_1, output_server_info_1) =
        get_output_server(&mut session, StopMessage::Polling(Polling::Enable));
    let output_client =
        run_client_probe(Server::Standard, &setup.settings.data.network.listen_socket);
    let (output_server_trce_2, output_server_info_2) =
        get_output_server(&mut session, StopMessage::Polling(Polling::Enable));
    let output_log = run_client_log(&setup.settings.data.network.listen_socket);

    mocks.iter().for_each(|mock| mock.assert());

    insta::assert_snapshot!(output_server_info_1, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> INFO probing server as we are in time
    <timestamp> INFO no update is current available for this device
    "###);

    insta::assert_snapshot!(output_server_info_2, @"<timestamp> INFO no update is current available for this device");

    insta::assert_snapshot!(output_server_trce_1, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> DEBG loading system settings from "<file>"
    <timestamp> DEBG runtime settings file "<file>" does not exists, using default settings
    <timestamp> TRCE starting to handle: entry_point
    <timestamp> DEBG polling is enabled
    <timestamp> TRCE starting to handle: poll
    <timestamp> INFO probing server as we are in time
    <timestamp> TRCE starting to handle: probe
    <timestamp> INFO no update is current available for this device
    <timestamp> DEBG updating last polling time
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> TRCE starting to handle: entry_point
    <timestamp> DEBG polling is enabled
    <timestamp> TRCE starting to handle: poll
    <timestamp> DEBG delaying <time> till next probe
    "###);

    insta::assert_snapshot!(output_server_trce_2, @r###"
    <timestamp> DEBG receiving probe request
    <timestamp> TRCE received external request: Probe(None)
    <timestamp> INFO no update is current available for this device
    <timestamp> DEBG updating last polling time
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> TRCE starting to handle: entry_point
    <timestamp> DEBG polling is enabled
    <timestamp> TRCE starting to handle: poll
    <timestamp> DEBG delaying <time> till next probe
    "###);

    insta::assert_snapshot!(remove_carriage_newline_characters(output_client), @r###"
    There are no updates available.
    "###);

    insta::assert_snapshot!(output_log, @r###"
    <timestamp> INFO no update is current available for this device
    <timestamp> DEBG updating last polling time
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> TRCE starting to handle: entry_point
    <timestamp> DEBG polling is enabled
    <timestamp> TRCE starting to handle: poll
    <timestamp> DEBG delaying <time> till next probe
    <timestamp> TRCE delaying transition for: <time>
    <timestamp> DEBG receiving log request
    "###);
}

#[test]
fn correct_config_no_update_no_polling_with_probe_api() {
    let mocks = create_mock_server(FakeServer::NoUpdate);
    let (mut session, setup) = Settings::default().timeout(300).init_server();

    let (output_server_trce_1, output_server_info_1) =
        get_output_server(&mut session, StopMessage::Polling(Polling::Disable));
    let output_client =
        run_client_probe(Server::Standard, &setup.settings.data.network.listen_socket);
    let (output_server_trce_2, output_server_info_2) =
        get_output_server(&mut session, StopMessage::Polling(Polling::Disable));
    let output_log = run_client_log(&setup.settings.data.network.listen_socket);

    mocks.iter().for_each(|mock| mock.assert());

    insta::assert_snapshot!(output_server_info_1, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> INFO parking state machine
    "###);

    insta::assert_snapshot!(output_server_info_2, @r###"
    <timestamp> INFO no update is current available for this device
    <timestamp> INFO parking state machine
    "###);

    insta::assert_snapshot!(output_server_trce_1, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> DEBG loading system settings from "<file>"
    <timestamp> DEBG runtime settings file "<file>" does not exists, using default settings
    <timestamp> TRCE starting to handle: entry_point
    <timestamp> DEBG polling is disabled
    <timestamp> TRCE starting to handle: park
    <timestamp> INFO parking state machine
    "###);

    insta::assert_snapshot!(output_server_trce_2, @r###"

    <timestamp> DEBG receiving probe request
    <timestamp> TRCE received external request: Probe(None)
    <timestamp> INFO no update is current available for this device
    <timestamp> DEBG updating last polling time
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> TRCE starting to handle: entry_point
    <timestamp> DEBG polling is disabled
    <timestamp> TRCE starting to handle: park
    <timestamp> INFO parking state machine
    "###);

    insta::assert_snapshot!(remove_carriage_newline_characters(output_client), @r###"
    There are no updates available.
    "###);

    insta::assert_snapshot!(output_log, @r###"
    <timestamp> INFO no update is current available for this device
    <timestamp> DEBG updating last polling time
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> TRCE starting to handle: entry_point
    <timestamp> DEBG polling is disabled
    <timestamp> TRCE starting to handle: park
    <timestamp> INFO parking state machine
    "###);
}

#[test]
fn correct_config_update_polling() {
    let (mut session, setup) = Settings::default().timeout(300).polling().init_server();
    let mocks = create_mock_server(FakeServer::HasUpdate(setup.firmware.data.product_uid.clone()));

    let (output_server_trce, output_server_info) =
        get_output_server(&mut session, StopMessage::Polling(Polling::Enable));
    let output_log = run_client_log(&setup.settings.data.network.listen_socket);

    mocks.iter().for_each(|mock| mock.assert());

    insta::assert_snapshot!(output_server_info, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> INFO probing server as we are in time
    <timestamp> INFO update received: 1.2 (87effe73b80453f397cee4db3c3589a8630b220876dff8fb23447315037ff96d)
    <timestamp> INFO no signature key available on device, ignoring signature validation
    <timestamp> INFO installing update: 1.2 (87effe73b80453f397cee4db3c3589a8630b220876dff8fb23447315037ff96d)
    <timestamp> INFO using installation set as target 1
    <timestamp> INFO swapping active installation set
    <timestamp> INFO update installed successfully
    <timestamp> INFO triggering reboot
    "###);

    insta::assert_snapshot!(output_server_trce, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> DEBG loading system settings from "<file>"
    <timestamp> DEBG runtime settings file "<file>" does not exists, using default settings
    <timestamp> TRCE starting to handle: entry_point
    <timestamp> DEBG polling is enabled
    <timestamp> TRCE starting to handle: poll
    <timestamp> INFO probing server as we are in time
    <timestamp> TRCE starting to handle: probe
    <timestamp> DEBG updating last polling time
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> INFO update received: 1.2 (87effe73b80453f397cee4db3c3589a8630b220876dff8fb23447315037ff96d)
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
    <timestamp> INFO installing update: 1.2 (87effe73b80453f397cee4db3c3589a8630b220876dff8fb23447315037ff96d)
    <timestamp> INFO using installation set as target 1
    <timestamp> DEBG marking package 87effe73b80453f397cee4db3c3589a8630b220876dff8fb23447315037ff96d as installed
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> DEBG setting upgrading to 1
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> INFO swapping active installation set
    <timestamp> INFO update installed successfully
    <timestamp> TRCE starting to handle: reboot
    <timestamp> INFO triggering reboot
    <timestamp> TRCE starting to handle: entry_point
    <timestamp> DEBG polling is enabled
    <timestamp> TRCE starting to handle: poll
    <timestamp> DEBG delaying <time> till next probe
    "###);

    insta::assert_snapshot!(output_log, @r###"
    <timestamp> INFO update received: 1.2 (87effe73b80453f397cee4db3c3589a8630b220876dff8fb23447315037ff96d)
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
    <timestamp> INFO installing update: 1.2 (87effe73b80453f397cee4db3c3589a8630b220876dff8fb23447315037ff96d)
    <timestamp> INFO using installation set as target 1
    <timestamp> DEBG marking package 87effe73b80453f397cee4db3c3589a8630b220876dff8fb23447315037ff96d as installed
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> DEBG setting upgrading to 1
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> INFO swapping active installation set
    <timestamp> INFO update installed successfully
    <timestamp> TRCE starting to handle: reboot
    <timestamp> INFO triggering reboot
    <timestamp> TRCE starting to handle: entry_point
    <timestamp> DEBG polling is enabled
    <timestamp> TRCE starting to handle: poll
    <timestamp> DEBG delaying <time> till next probe
    <timestamp> TRCE delaying transition for: <time>
    <timestamp> DEBG receiving log request
    "###);
}

#[test]
fn correct_config_statechange_callback() {
    let state_change_script = r#"#! /bin/bash
[ "$1" = "download" ] && echo "cancel" || echo
"#;

    let (mut session, setup) = Settings::default()
        .timeout(300)
        .polling()
        .state_change_callback(state_change_script)
        .init_server();
    let _mocks = create_mock_server(FakeServer::HasUpdate(setup.firmware.data.product_uid.clone()));

    let (output_server_trce, output_server_info) =
        get_output_server(&mut session, StopMessage::Polling(Polling::Enable));
    let output_log = run_client_log(&setup.settings.data.network.listen_socket);

    insta::assert_snapshot!(output_server_info, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> INFO probing server as we are in time
    <timestamp> INFO update received: 1.2 (87effe73b80453f397cee4db3c3589a8630b220876dff8fb23447315037ff96d)
    <timestamp> INFO no signature key available on device, ignoring signature validation
    <timestamp> INFO cancelling transition to 'download' due to state change callback request
    "###);

    insta::assert_snapshot!(output_server_trce, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> DEBG loading system settings from "<file>"
    <timestamp> DEBG runtime settings file "<file>" does not exists, using default settings
    <timestamp> TRCE starting to handle: entry_point
    <timestamp> DEBG polling is enabled
    <timestamp> TRCE starting to handle: poll
    <timestamp> INFO probing server as we are in time
    <timestamp> TRCE starting to handle: probe
    <timestamp> DEBG updating last polling time
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> INFO update received: 1.2 (87effe73b80453f397cee4db3c3589a8630b220876dff8fb23447315037ff96d)
    <timestamp> TRCE starting to handle: validation
    <timestamp> INFO no signature key available on device, ignoring signature validation
    <timestamp> TRCE starting to handle: download
    <timestamp> INFO cancelling transition to 'download' due to state change callback request
    <timestamp> TRCE starting to handle: entry_point
    <timestamp> DEBG polling is enabled
    <timestamp> TRCE starting to handle: poll
    <timestamp> DEBG delaying <time> till next probe
    "###);

    insta::assert_snapshot!(output_log, @r###"
    <timestamp> INFO update received: 1.2 (87effe73b80453f397cee4db3c3589a8630b220876dff8fb23447315037ff96d)
    <timestamp> TRCE starting to handle: validation
    <timestamp> INFO no signature key available on device, ignoring signature validation
    <timestamp> TRCE starting to handle: download
    <timestamp> INFO cancelling transition to 'download' due to state change callback request
    <timestamp> TRCE starting to handle: entry_point
    <timestamp> DEBG polling is enabled
    <timestamp> TRCE starting to handle: poll
    <timestamp> DEBG delaying <time> till next probe
    <timestamp> TRCE delaying transition for: <time>
    <timestamp> DEBG receiving log request
    "###);
}

#[test]
fn correct_config_error_state_callback() {
    let state_change_script = r#"#! /bin/bash
[ "$1" = "error" ] && echo "cancel" || echo
"#;

    let (mut session, setup) = Settings::default()
        .timeout(300)
        .polling()
        .state_change_callback(state_change_script)
        .init_server();
    let _mocks = create_mock_server(FakeServer::CheckRequirementsTest(
        setup.firmware.data.product_uid.clone(),
    ));

    let (output_server_trce, output_server_info) =
        get_output_server(&mut session, StopMessage::Polling(Polling::Enable));
    let output_log = run_client_log(&setup.settings.data.network.listen_socket);

    insta::assert_snapshot!(output_server_info, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> INFO probing server as we are in time
    <timestamp> INFO update received: 1.2 (fb21b217cb83e8af368c773eb13bad0a94e1b0088c6bf561072decf3c1ae9df3)
    <timestamp> INFO no signature key available on device, ignoring signature validation
    <timestamp> ERRO update package: 1.2 (fb21b217cb83e8af368c773eb13bad0a94e1b0088c6bf561072decf3c1ae9df3) has failed to meet the install requirements
    <timestamp> INFO cancelling transition to 'error' due to state change callback request
    "###);

    insta::assert_snapshot!(output_server_trce, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> DEBG loading system settings from "<file>"
    <timestamp> DEBG runtime settings file "<file>" does not exists, using default settings
    <timestamp> TRCE starting to handle: entry_point
    <timestamp> DEBG polling is enabled
    <timestamp> TRCE starting to handle: poll
    <timestamp> INFO probing server as we are in time
    <timestamp> TRCE starting to handle: probe
    <timestamp> DEBG updating last polling time
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> INFO update received: 1.2 (fb21b217cb83e8af368c773eb13bad0a94e1b0088c6bf561072decf3c1ae9df3)
    <timestamp> TRCE starting to handle: validation
    <timestamp> INFO no signature key available on device, ignoring signature validation
    <timestamp> ERRO update package: 1.2 (fb21b217cb83e8af368c773eb13bad0a94e1b0088c6bf561072decf3c1ae9df3) has failed to meet the install requirements
    <timestamp> TRCE starting to handle: error
    <timestamp> INFO cancelling transition to 'error' due to state change callback request
    <timestamp> TRCE starting to handle: entry_point
    <timestamp> DEBG polling is enabled
    <timestamp> TRCE starting to handle: poll
    <timestamp> DEBG delaying <time> till next probe
    "###);

    insta::assert_snapshot!(output_log, @r###"
    <timestamp> INFO update received: 1.2 (fb21b217cb83e8af368c773eb13bad0a94e1b0088c6bf561072decf3c1ae9df3)
    <timestamp> TRCE starting to handle: validation
    <timestamp> INFO no signature key available on device, ignoring signature validation
    <timestamp> ERRO update package: 1.2 (fb21b217cb83e8af368c773eb13bad0a94e1b0088c6bf561072decf3c1ae9df3) has failed to meet the install requirements
    <timestamp> TRCE starting to handle: error
    <timestamp> INFO cancelling transition to 'error' due to state change callback request
    <timestamp> TRCE starting to handle: entry_point
    <timestamp> DEBG polling is enabled
    <timestamp> TRCE starting to handle: poll
    <timestamp> DEBG delaying <time> till next probe
    <timestamp> TRCE delaying transition for: <time>
    <timestamp> DEBG receiving log request
    "###);
}
