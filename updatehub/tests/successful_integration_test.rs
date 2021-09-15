// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use common::{
    create_mock_server, get_output_server, remove_carriage_newline_characters,
    run_client_local_install, run_client_log, run_client_probe, FakeServer, Polling, Server,
    Settings, StopMessage,
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
    <timestamp> TRCE starting to handle 'entry_point' state
    <timestamp> DEBG polling is disabled
    <timestamp> TRCE starting to handle 'park' state
    <timestamp> INFO parking state machine
    "###);

    insta::assert_snapshot!(output_log, @r###"
    <timestamp> DEBG loading system settings from "<file>"
    <timestamp> DEBG runtime settings file "<file>" does not exists, using default settings
    <timestamp> TRCE starting to handle 'entry_point' state
    <timestamp> DEBG polling is disabled
    <timestamp> TRCE starting to handle 'park' state
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
    <timestamp> TRCE starting to handle 'entry_point' state
    <timestamp> DEBG polling is enabled
    <timestamp> TRCE starting to handle 'poll' state
    <timestamp> INFO probing server as we are in time
    <timestamp> TRCE starting to handle 'probe' state
    <timestamp> INFO no update is current available for this device
    <timestamp> DEBG updating last polling time
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> TRCE starting to handle 'entry_point' state
    <timestamp> DEBG polling is enabled
    <timestamp> TRCE starting to handle 'poll' state
    <timestamp> DEBG delaying <time> till next probe
    "###);

    insta::assert_snapshot!(output_log, @r###"
    <timestamp> DEBG loading system settings from "<file>"
    <timestamp> DEBG runtime settings file "<file>" does not exists, using default settings
    <timestamp> TRCE starting to handle 'entry_point' state
    <timestamp> DEBG polling is enabled
    <timestamp> TRCE starting to handle 'poll' state
    <timestamp> INFO probing server as we are in time
    <timestamp> TRCE starting to handle 'probe' state
    <timestamp> INFO no update is current available for this device
    <timestamp> DEBG updating last polling time
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> TRCE starting to handle 'entry_point' state
    <timestamp> DEBG polling is enabled
    <timestamp> TRCE starting to handle 'poll' state
    <timestamp> DEBG delaying <time> till next probe
    <timestamp> TRCE delaying transition for: <time>
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

    insta::assert_snapshot!(output_server_info_2, @r###"
    <timestamp> INFO Probing the server as requested by the user
    <timestamp> INFO no update is current available for this device
    "###);

    insta::assert_snapshot!(output_server_trce_1, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> DEBG loading system settings from "<file>"
    <timestamp> DEBG runtime settings file "<file>" does not exists, using default settings
    <timestamp> TRCE starting to handle 'entry_point' state
    <timestamp> DEBG polling is enabled
    <timestamp> TRCE starting to handle 'poll' state
    <timestamp> INFO probing server as we are in time
    <timestamp> TRCE starting to handle 'probe' state
    <timestamp> INFO no update is current available for this device
    <timestamp> DEBG updating last polling time
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> TRCE starting to handle 'entry_point' state
    <timestamp> DEBG polling is enabled
    <timestamp> TRCE starting to handle 'poll' state
    <timestamp> DEBG delaying <time> till next probe
    "###);

    insta::assert_snapshot!(output_server_trce_2, @r###"
    <timestamp> DEBG receiving probe request
    <timestamp> TRCE received external request: Probe(None)
    <timestamp> INFO Probing the server as requested by the user
    <timestamp> INFO no update is current available for this device
    <timestamp> DEBG updating last polling time
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> TRCE starting to handle 'entry_point' state
    <timestamp> DEBG polling is enabled
    <timestamp> TRCE starting to handle 'poll' state
    <timestamp> DEBG delaying <time> till next probe
    "###);

    insta::assert_snapshot!(remove_carriage_newline_characters(output_client), @r###"
    There are no updates available.
    "###);

    insta::assert_snapshot!(output_log, @r###"
    <timestamp> INFO Probing the server as requested by the user
    <timestamp> INFO no update is current available for this device
    <timestamp> DEBG updating last polling time
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> TRCE starting to handle 'entry_point' state
    <timestamp> DEBG polling is enabled
    <timestamp> TRCE starting to handle 'poll' state
    <timestamp> DEBG delaying <time> till next probe
    <timestamp> TRCE delaying transition for: <time>
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
    <timestamp> INFO Probing the server as requested by the user
    <timestamp> INFO no update is current available for this device
    <timestamp> INFO parking state machine
    "###);

    insta::assert_snapshot!(output_server_trce_1, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> DEBG loading system settings from "<file>"
    <timestamp> DEBG runtime settings file "<file>" does not exists, using default settings
    <timestamp> TRCE starting to handle 'entry_point' state
    <timestamp> DEBG polling is disabled
    <timestamp> TRCE starting to handle 'park' state
    <timestamp> INFO parking state machine
    "###);

    insta::assert_snapshot!(output_server_trce_2, @r###"
    <timestamp> DEBG receiving probe request
    <timestamp> TRCE received external request: Probe(None)
    <timestamp> INFO Probing the server as requested by the user
    <timestamp> INFO no update is current available for this device
    <timestamp> DEBG updating last polling time
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> TRCE starting to handle 'entry_point' state
    <timestamp> DEBG polling is disabled
    <timestamp> TRCE starting to handle 'park' state
    <timestamp> INFO parking state machine
    "###);

    insta::assert_snapshot!(remove_carriage_newline_characters(output_client), @r###"
    There are no updates available.
    "###);

    insta::assert_snapshot!(output_log, @r###"
    <timestamp> INFO Probing the server as requested by the user
    <timestamp> INFO no update is current available for this device
    <timestamp> DEBG updating last polling time
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> TRCE starting to handle 'entry_point' state
    <timestamp> DEBG polling is disabled
    <timestamp> TRCE starting to handle 'park' state
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
    <timestamp> TRCE starting to handle 'entry_point' state
    <timestamp> DEBG polling is enabled
    <timestamp> TRCE starting to handle 'poll' state
    <timestamp> INFO probing server as we are in time
    <timestamp> TRCE starting to handle 'probe' state
    <timestamp> DEBG updating last polling time
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> INFO update received: 1.2 (87effe73b80453f397cee4db3c3589a8630b220876dff8fb23447315037ff96d)
    <timestamp> TRCE starting to handle 'validation' state
    <timestamp> INFO no signature key available on device, ignoring signature validation
    <timestamp> TRCE starting to handle 'download' state
    <timestamp> TRCE the following objects are missing: [("testfile", "23c3c412177bd37b9b61bf4738b18dc1fe003811c2583a14d2d9952d8b6a75b4")]
    <timestamp> DEBG starting download of: testfile (23c3c412177bd37b9b61bf4738b18dc1fe003811c2583a14d2d9952d8b6a75b4)
    <timestamp> DEBG <percentage>% of the file has been downloaded
    <timestamp> DEBG <percentage>% of the file has been downloaded
    <timestamp> DEBG 100% of the file has been downloaded
    <timestamp> TRCE starting to handle 'install' state
    <timestamp> INFO installing update: 1.2 (87effe73b80453f397cee4db3c3589a8630b220876dff8fb23447315037ff96d)
    <timestamp> INFO using installation set as target 1
    <timestamp> DEBG marking package 87effe73b80453f397cee4db3c3589a8630b220876dff8fb23447315037ff96d as installed
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> DEBG setting upgrading to 1
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> INFO swapping active installation set
    <timestamp> INFO update installed successfully
    <timestamp> TRCE starting to handle 'reboot' state
    <timestamp> INFO triggering reboot
    <timestamp> TRCE starting to handle 'entry_point' state
    <timestamp> DEBG polling is enabled
    <timestamp> TRCE starting to handle 'poll' state
    <timestamp> DEBG delaying <time> till next probe
    "###);

    insta::assert_snapshot!(output_log, @r###"
    <timestamp> INFO update received: 1.2 (87effe73b80453f397cee4db3c3589a8630b220876dff8fb23447315037ff96d)
    <timestamp> TRCE starting to handle 'validation' state
    <timestamp> INFO no signature key available on device, ignoring signature validation
    <timestamp> TRCE starting to handle 'download' state
    <timestamp> TRCE the following objects are missing: [("testfile", "23c3c412177bd37b9b61bf4738b18dc1fe003811c2583a14d2d9952d8b6a75b4")]
    <timestamp> DEBG starting download of: testfile (23c3c412177bd37b9b61bf4738b18dc1fe003811c2583a14d2d9952d8b6a75b4)
    <timestamp> DEBG <percentage>% of the file has been downloaded
    <timestamp> DEBG <percentage>% of the file has been downloaded
    <timestamp> DEBG 100% of the file has been downloaded
    <timestamp> TRCE starting to handle 'install' state
    <timestamp> INFO installing update: 1.2 (87effe73b80453f397cee4db3c3589a8630b220876dff8fb23447315037ff96d)
    <timestamp> INFO using installation set as target 1
    <timestamp> DEBG marking package 87effe73b80453f397cee4db3c3589a8630b220876dff8fb23447315037ff96d as installed
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> DEBG setting upgrading to 1
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> INFO swapping active installation set
    <timestamp> INFO update installed successfully
    <timestamp> TRCE starting to handle 'reboot' state
    <timestamp> INFO triggering reboot
    <timestamp> TRCE starting to handle 'entry_point' state
    <timestamp> DEBG polling is enabled
    <timestamp> TRCE starting to handle 'poll' state
    <timestamp> DEBG delaying <time> till next probe
    <timestamp> TRCE delaying transition for: <time>
    "###);
}

#[test]
fn correct_config_statechange_callback() {
    let state_change_script = r#"#! /bin/sh
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
    <timestamp> INFO running state change callback for 'probe' state
    <timestamp> INFO probe callback has exit with success
    <timestamp> INFO update received: 1.2 (87effe73b80453f397cee4db3c3589a8630b220876dff8fb23447315037ff96d)
    <timestamp> INFO no signature key available on device, ignoring signature validation
    <timestamp> INFO running state change callback for 'download' state
    <timestamp> INFO download callback has exit with success
    <timestamp> INFO cancelling transition to 'download' due to state change callback request
    "###);

    insta::assert_snapshot!(output_server_trce, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> DEBG loading system settings from "<file>"
    <timestamp> DEBG runtime settings file "<file>" does not exists, using default settings
    <timestamp> TRCE starting to handle 'entry_point' state
    <timestamp> DEBG polling is enabled
    <timestamp> TRCE starting to handle 'poll' state
    <timestamp> INFO probing server as we are in time
    <timestamp> TRCE starting to handle 'probe' state
    <timestamp> INFO running state change callback for 'probe' state
    <timestamp> INFO probe callback has exit with success
    <timestamp> DEBG updating last polling time
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> INFO update received: 1.2 (87effe73b80453f397cee4db3c3589a8630b220876dff8fb23447315037ff96d)
    <timestamp> TRCE starting to handle 'validation' state
    <timestamp> INFO no signature key available on device, ignoring signature validation
    <timestamp> TRCE starting to handle 'download' state
    <timestamp> INFO running state change callback for 'download' state
    <timestamp> INFO download callback has exit with success
    <timestamp> INFO cancelling transition to 'download' due to state change callback request
    <timestamp> TRCE starting to handle 'entry_point' state
    <timestamp> DEBG polling is enabled
    <timestamp> TRCE starting to handle 'poll' state
    <timestamp> DEBG delaying <time> till next probe
    "###);

    insta::assert_snapshot!(output_log, @r###"
    <timestamp> INFO update received: 1.2 (87effe73b80453f397cee4db3c3589a8630b220876dff8fb23447315037ff96d)
    <timestamp> TRCE starting to handle 'validation' state
    <timestamp> INFO no signature key available on device, ignoring signature validation
    <timestamp> TRCE starting to handle 'download' state
    <timestamp> INFO running state change callback for 'download' state
    <timestamp> INFO download callback has exit with success
    <timestamp> INFO cancelling transition to 'download' due to state change callback request
    <timestamp> TRCE starting to handle 'entry_point' state
    <timestamp> DEBG polling is enabled
    <timestamp> TRCE starting to handle 'poll' state
    <timestamp> DEBG delaying <time> till next probe
    <timestamp> TRCE delaying transition for: <time>
    "###);
}

#[test]
fn correct_config_error_state_callback() {
    let state_change_script = r#"#! /bin/sh
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
    <timestamp> INFO running state change callback for 'probe' state
    <timestamp> INFO probe callback has exit with success
    <timestamp> INFO update received: 1.2 (fb21b217cb83e8af368c773eb13bad0a94e1b0088c6bf561072decf3c1ae9df3)
    <timestamp> INFO no signature key available on device, ignoring signature validation
    <timestamp> ERRO update package: 1.2 (fb21b217cb83e8af368c773eb13bad0a94e1b0088c6bf561072decf3c1ae9df3) has failed to meet the install requirements
    <timestamp> INFO running state change callback for 'error' state
    <timestamp> INFO error callback has exit with success
    <timestamp> INFO cancelling transition to 'error' due to state change callback request
    "###);

    insta::assert_snapshot!(output_server_trce, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> DEBG loading system settings from "<file>"
    <timestamp> DEBG runtime settings file "<file>" does not exists, using default settings
    <timestamp> TRCE starting to handle 'entry_point' state
    <timestamp> DEBG polling is enabled
    <timestamp> TRCE starting to handle 'poll' state
    <timestamp> INFO probing server as we are in time
    <timestamp> TRCE starting to handle 'probe' state
    <timestamp> INFO running state change callback for 'probe' state
    <timestamp> INFO probe callback has exit with success
    <timestamp> DEBG updating last polling time
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> INFO update received: 1.2 (fb21b217cb83e8af368c773eb13bad0a94e1b0088c6bf561072decf3c1ae9df3)
    <timestamp> TRCE starting to handle 'validation' state
    <timestamp> INFO no signature key available on device, ignoring signature validation
    <timestamp> ERRO update package: 1.2 (fb21b217cb83e8af368c773eb13bad0a94e1b0088c6bf561072decf3c1ae9df3) has failed to meet the install requirements
    <timestamp> TRCE starting to handle 'error' state
    <timestamp> INFO running state change callback for 'error' state
    <timestamp> INFO error callback has exit with success
    <timestamp> INFO cancelling transition to 'error' due to state change callback request
    <timestamp> TRCE starting to handle 'entry_point' state
    <timestamp> DEBG polling is enabled
    <timestamp> TRCE starting to handle 'poll' state
    <timestamp> DEBG delaying <time> till next probe
    "###);

    insta::assert_snapshot!(output_log, @r###"
    <timestamp> INFO update received: 1.2 (fb21b217cb83e8af368c773eb13bad0a94e1b0088c6bf561072decf3c1ae9df3)
    <timestamp> TRCE starting to handle 'validation' state
    <timestamp> INFO no signature key available on device, ignoring signature validation
    <timestamp> ERRO update package: 1.2 (fb21b217cb83e8af368c773eb13bad0a94e1b0088c6bf561072decf3c1ae9df3) has failed to meet the install requirements
    <timestamp> TRCE starting to handle 'error' state
    <timestamp> INFO running state change callback for 'error' state
    <timestamp> INFO error callback has exit with success
    <timestamp> INFO cancelling transition to 'error' due to state change callback request
    <timestamp> TRCE starting to handle 'entry_point' state
    <timestamp> DEBG polling is enabled
    <timestamp> TRCE starting to handle 'poll' state
    <timestamp> DEBG delaying <time> till next probe
    <timestamp> TRCE delaying transition for: <time>
    "###);
}

#[test]
fn correct_config_remote_install() {
    let mocks = create_mock_server(FakeServer::RemoteInstall);
    let (mut session, setup) = Settings::default().timeout(300).init_server();

    let (output_server_trce_1, output_server_info_1) =
        get_output_server(&mut session, StopMessage::Polling(Polling::Disable));
    let output_client = run_client_local_install(
        &mockito::server_url(),
        &setup.settings.data.network.listen_socket,
    );
    let (output_server_trce_2, output_server_info_2) =
        get_output_server(&mut session, StopMessage::Polling(Polling::Disable));
    let output_log = run_client_log(&setup.settings.data.network.listen_socket);

    mocks.iter().for_each(|mock| mock.assert());

    insta::assert_snapshot!(output_server_info_1, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> INFO parking state machine
    "###);

    insta::assert_snapshot!(output_server_info_2, @r###"
    <timestamp> INFO fetching update package directly from url: "http://127.0.0.1:1234/some-direct-package-url"
    <timestamp> INFO installing local package: "<file>"
    <timestamp> INFO update package extracted: fake-test-package-01 (ab99ebb6afd75cf9e51c409cbf63daa7297446721ea75c6dffcbb84c2692dd62)
    <timestamp> INFO installing update: fake-test-package-01 (ab99ebb6afd75cf9e51c409cbf63daa7297446721ea75c6dffcbb84c2692dd62)
    <timestamp> INFO using installation set as target 1
    <timestamp> INFO swapping active installation set
    <timestamp> INFO update installed successfully
    <timestamp> INFO triggering reboot
    <timestamp> INFO parking state machine
    "###);

    insta::assert_snapshot!(output_server_trce_1, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> DEBG loading system settings from "<file>"
    <timestamp> DEBG runtime settings file "<file>" does not exists, using default settings
    <timestamp> TRCE starting to handle 'entry_point' state
    <timestamp> DEBG polling is disabled
    <timestamp> TRCE starting to handle 'park' state
    <timestamp> INFO parking state machine
    "###);

    insta::assert_snapshot!(output_server_trce_2, @r###"
    <timestamp> DEBG receiving remote_install request
    <timestamp> TRCE Remote install requested
    <timestamp> TRCE received external request: RemoteInstall("http://127.0.0.1:1234/some-direct-package-url")
    <timestamp> TRCE starting to handle 'direct_download' state
    <timestamp> INFO fetching update package directly from url: "http://127.0.0.1:1234/some-direct-package-url"
    <timestamp> DEBG 100% of the file has been downloaded
    <timestamp> TRCE starting to handle 'prepare_local_install' state
    <timestamp> INFO installing local package: "<file>"
    <timestamp> DEBG successfuly uncompressed metadata file
    <timestamp> INFO update package extracted: fake-test-package-01 (ab99ebb6afd75cf9e51c409cbf63daa7297446721ea75c6dffcbb84c2692dd62)
    <timestamp> TRCE starting to handle 'install' state
    <timestamp> INFO installing update: fake-test-package-01 (ab99ebb6afd75cf9e51c409cbf63daa7297446721ea75c6dffcbb84c2692dd62)
    <timestamp> INFO using installation set as target 1
    <timestamp> DEBG marking package ab99ebb6afd75cf9e51c409cbf63daa7297446721ea75c6dffcbb84c2692dd62 as installed
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> DEBG setting upgrading to 1
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> INFO swapping active installation set
    <timestamp> INFO update installed successfully
    <timestamp> TRCE starting to handle 'reboot' state
    <timestamp> INFO triggering reboot
    <timestamp> TRCE starting to handle 'entry_point' state
    <timestamp> DEBG polling is disabled
    <timestamp> TRCE starting to handle 'park' state
    <timestamp> INFO parking state machine
    "###);

    insta::assert_snapshot!(remove_carriage_newline_characters(output_client), @r###"
    Local install request accepted from Park state
    Run 'updatehub client log --watch' to follow the log's progress
    "###);

    insta::assert_snapshot!(output_log, @r###"
    <timestamp> TRCE starting to handle 'direct_download' state
    <timestamp> INFO fetching update package directly from url: "http://127.0.0.1:1234/some-direct-package-url"
    <timestamp> DEBG 100% of the file has been downloaded
    <timestamp> TRCE starting to handle 'prepare_local_install' state
    <timestamp> INFO installing local package: "<file>"
    <timestamp> DEBG successfuly uncompressed metadata file
    <timestamp> INFO update package extracted: fake-test-package-01 (ab99ebb6afd75cf9e51c409cbf63daa7297446721ea75c6dffcbb84c2692dd62)
    <timestamp> TRCE starting to handle 'install' state
    <timestamp> INFO installing update: fake-test-package-01 (ab99ebb6afd75cf9e51c409cbf63daa7297446721ea75c6dffcbb84c2692dd62)
    <timestamp> INFO using installation set as target 1
    <timestamp> DEBG marking package ab99ebb6afd75cf9e51c409cbf63daa7297446721ea75c6dffcbb84c2692dd62 as installed
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> DEBG setting upgrading to 1
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> INFO swapping active installation set
    <timestamp> INFO update installed successfully
    <timestamp> TRCE starting to handle 'reboot' state
    <timestamp> INFO triggering reboot
    <timestamp> TRCE starting to handle 'entry_point' state
    <timestamp> DEBG polling is disabled
    <timestamp> TRCE starting to handle 'park' state
    <timestamp> INFO parking state machine
    "###);
}

#[test]
fn validation_callback() {
    let validate_script = r#"#! /bin/sh
exit 0
"#;

    // Even tho we don't update we start the mock for the probe performed on update
    let mocks = create_mock_server(FakeServer::NoUpdate);
    let (mut session, setup) = Settings::default()
        .timeout(300)
        .validate_callback(validate_script)
        .booting_from_update()
        .init_server();

    let (output_server_trce, output_server_info) =
        get_output_server(&mut session, StopMessage::Polling(Polling::Disable));
    let output_log = run_client_log(&setup.settings.data.network.listen_socket);

    mocks.iter().for_each(|mock| mock.assert());

    insta::assert_snapshot!(output_server_info, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> INFO booting from a recent installation
    <timestamp> INFO running validate callback
    <timestamp> INFO validate callback has exit with success
    <timestamp> INFO triggering Probe to finish update
    <timestamp> INFO no update is current available for this device
    <timestamp> INFO parking state machine
    "###);

    insta::assert_snapshot!(output_server_trce, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> DEBG loading system settings from "<file>"
    <timestamp> DEBG loading runtime settings from "<file>"
    <timestamp> INFO booting from a recent installation
    <timestamp> INFO running validate callback
    <timestamp> INFO validate callback has exit with success
    <timestamp> DEBG reseting installation settings
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> TRCE starting to handle 'entry_point' state
    <timestamp> INFO triggering Probe to finish update
    <timestamp> DEBG disabling foce poll
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> TRCE starting to handle 'probe' state
    <timestamp> INFO no update is current available for this device
    <timestamp> DEBG updating last polling time
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> TRCE starting to handle 'entry_point' state
    <timestamp> DEBG polling is disabled
    <timestamp> TRCE starting to handle 'park' state
    <timestamp> INFO parking state machine
    "###);

    insta::assert_snapshot!(output_log, @r###"
    <timestamp> DEBG loading system settings from "<file>"
    <timestamp> DEBG loading runtime settings from "<file>"
    <timestamp> INFO booting from a recent installation
    <timestamp> INFO running validate callback
    <timestamp> INFO validate callback has exit with success
    <timestamp> DEBG reseting installation settings
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> TRCE starting to handle 'entry_point' state
    <timestamp> INFO triggering Probe to finish update
    <timestamp> DEBG disabling foce poll
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> TRCE starting to handle 'probe' state
    <timestamp> INFO no update is current available for this device
    <timestamp> DEBG updating last polling time
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> TRCE starting to handle 'entry_point' state
    <timestamp> DEBG polling is disabled
    <timestamp> TRCE starting to handle 'park' state
    <timestamp> INFO parking state machine
    "###);
}

#[test]
fn failed_validation_callback() {
    let validate_script = r#"#! /bin/sh
exit 1
"#;

    let (mut session, _setup) = Settings::default()
        .timeout(300)
        .validate_callback(validate_script)
        .booting_from_update()
        .init_server();

    let (output_server_trce, output_server_info) = get_output_server(
        &mut session,
        StopMessage::Custom(
            r#"\r\n.* WARN swapped active installation set and running rollback"#.to_string(),
        ),
    );

    insta::assert_snapshot!(output_server_info, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> INFO booting from a recent installation
    <timestamp> INFO running validate callback
    <timestamp> ERRO validate callback has failed with status: ExitStatus(ExitStatus(256))
    <timestamp> WARN validate callback has failed
    "###);

    insta::assert_snapshot!(output_server_trce, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> DEBG loading system settings from "<file>"
    <timestamp> DEBG loading runtime settings from "<file>"
    <timestamp> INFO booting from a recent installation
    <timestamp> INFO running validate callback
    <timestamp> ERRO validate callback has failed with status: ExitStatus(ExitStatus(256))
    <timestamp> WARN validate callback has failed
    "###);
}

#[test]
#[cfg(feature = "v1-parsing")]
fn v1_validation_callback() {
    let validate_script = r#"#! /bin/sh
exit 0
"#;

    // Even tho we don't update we start the mock for the probe performed on update
    let mocks = create_mock_server(FakeServer::NoUpdate);
    let (mut session, setup) = Settings::default()
        .timeout(300)
        .validate_callback(validate_script)
        .booting_from_update()
        .init_server();

    // Overwrite runtimesettings with a v1 model
    std::fs::write(
        &setup.runtime_settings.stored_path,
        r#"
[Polling]
LastPoll=2021-06-01T14:38:57-03:00
FirstPoll=2021-05-01T13:33:33-03:00
ExtraInterval=0
Retries=0
ProbeASAP=false

[Update]
UpgradeToInstallation=1
"#,
    )
    .unwrap();

    let (output_server_trce, output_server_info) =
        get_output_server(&mut session, StopMessage::Polling(Polling::Disable));
    let output_log = run_client_log(&setup.settings.data.network.listen_socket);

    mocks.iter().for_each(|mock| mock.assert());

    insta::assert_snapshot!(output_server_info, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> WARN loaded v1 runtime settings successfully
    <timestamp> INFO booting from a recent installation
    <timestamp> INFO triggering Probe to finish update
    <timestamp> INFO no update is current available for this device
    <timestamp> INFO parking state machine
    "###);

    insta::assert_snapshot!(output_server_trce, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> DEBG loading system settings from "<file>"
    <timestamp> DEBG loading runtime settings from "<file>"
    <timestamp> WARN loaded v1 runtime settings successfully
    <timestamp> INFO booting from a recent installation
    <timestamp> DEBG reseting installation settings
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> TRCE starting to handle 'entry_point' state
    <timestamp> INFO triggering Probe to finish update
    <timestamp> DEBG disabling foce poll
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> TRCE starting to handle 'probe' state
    <timestamp> INFO no update is current available for this device
    <timestamp> DEBG updating last polling time
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> TRCE starting to handle 'entry_point' state
    <timestamp> DEBG polling is disabled
    <timestamp> TRCE starting to handle 'park' state
    <timestamp> INFO parking state machine
    "###);

    insta::assert_snapshot!(output_log, @r###"
    <timestamp> DEBG loading system settings from "<file>"
    <timestamp> DEBG loading runtime settings from "<file>"
    <timestamp> WARN loaded v1 runtime settings successfully
    <timestamp> INFO booting from a recent installation
    <timestamp> DEBG reseting installation settings
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> TRCE starting to handle 'entry_point' state
    <timestamp> INFO triggering Probe to finish update
    <timestamp> DEBG disabling foce poll
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> TRCE starting to handle 'probe' state
    <timestamp> INFO no update is current available for this device
    <timestamp> DEBG updating last polling time
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> TRCE starting to handle 'entry_point' state
    <timestamp> DEBG polling is disabled
    <timestamp> TRCE starting to handle 'park' state
    <timestamp> INFO parking state machine
    "###);
}

#[test]
#[cfg(feature = "v1-parsing")]
fn v1_failed_from_v1_validation_callback() {
    let validate_script = r#"#! /bin/sh
exit 1
"#;

    let (mut session, setup) = Settings::default()
        .timeout(300)
        .validate_callback(validate_script)
        .booting_from_update()
        .init_server();

    // Overwrite runtimesettings with a v1 model
    std::fs::write(
        &setup.runtime_settings.stored_path,
        r#"
[Polling]
LastPoll=2021-06-01T14:38:57-03:00
FirstPoll=2021-05-01T13:33:33-03:00
ExtraInterval=0
Retries=0
ProbeASAP=false

[Update]
UpgradeToInstallation=0
"#,
    )
    .unwrap();

    let (output_server_trce, output_server_info) = get_output_server(
        &mut session,
        StopMessage::Custom(
            r#"\r\n.* WARN swapped active installation set and running rollback"#.to_string(),
        ),
    );

    insta::assert_snapshot!(output_server_info, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> WARN loaded v1 runtime settings successfully
    <timestamp> INFO booting from a recent installation
    <timestamp> INFO running validate callback
    <timestamp> ERRO validate callback has failed with status: ExitStatus(ExitStatus(256))
    <timestamp> WARN validate callback has failed
    "###);

    insta::assert_snapshot!(output_server_trce, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> DEBG loading system settings from "<file>"
    <timestamp> DEBG loading runtime settings from "<file>"
    <timestamp> WARN loaded v1 runtime settings successfully
    <timestamp> INFO booting from a recent installation
    <timestamp> INFO running validate callback
    <timestamp> ERRO validate callback has failed with status: ExitStatus(ExitStatus(256))
    <timestamp> WARN validate callback has failed
    "###);
}

#[test]
#[cfg(feature = "v1-parsing")]
fn v1_failed_from_v2_validation_callback() {
    let validate_script = r#"#! /bin/sh
exit 1
"#;

    let (mut session, _setup) = Settings::default()
        .timeout(300)
        .validate_callback(validate_script)
        .booting_from_update()
        .init_server();

    let (output_server_trce, output_server_info) = get_output_server(
        &mut session,
        StopMessage::Custom(
            r#"\r\n.* WARN swapped active installation set and running rollback"#.to_string(),
        ),
    );

    insta::assert_snapshot!(output_server_info, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> INFO booting from a recent installation
    <timestamp> INFO running validate callback
    <timestamp> ERRO validate callback has failed with status: ExitStatus(ExitStatus(256))
    <timestamp> WARN validate callback has failed
    "###);

    insta::assert_snapshot!(output_server_trce, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> DEBG loading system settings from "<file>"
    <timestamp> DEBG loading runtime settings from "<file>"
    <timestamp> INFO booting from a recent installation
    <timestamp> INFO running validate callback
    <timestamp> ERRO validate callback has failed with status: ExitStatus(ExitStatus(256))
    <timestamp> WARN validate callback has failed
    "###);
}
