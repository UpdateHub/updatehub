// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use common::{
    create_mock_server, get_output_server, remove_carriage_newline_characters, rewrite_log_output,
    run_client_log, run_client_probe, FakeServer, Polling, Server, Settings, StopMessage,
};

pub mod common;

#[test]
fn failing_invalid_download_dir() {
    let tmp_dir = tempfile::tempdir().unwrap();
    let mut perms = std::fs::metadata(tmp_dir.path()).unwrap().permissions();
    perms.set_readonly(true);
    std::fs::set_permissions(tmp_dir.path(), perms).unwrap();

    let setup = Settings::default();
    let (mut session, setup) =
        setup.download_dir(tmp_dir.path().to_path_buf()).timeout(300).init_server();
    let _mocks = create_mock_server(FakeServer::HasUpdate(setup.firmware.data.product_uid.clone()));
    let output_server_1 = get_output_server(&mut session, StopMessage::Polling(Polling::Disable));

    let output_client =
        run_client_probe(Server::Standard, &setup.settings.data.network.listen_socket);
    let output_server_2 = get_output_server(&mut session, StopMessage::Polling(Polling::Disable));
    let output_log = run_client_log(&setup.settings.data.network.listen_socket);

    let (output_server_trce, output_server_info) = rewrite_log_output(output_server_1);

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

    insta::assert_snapshot!(rewrite_log_output(output_server_2).0.trim(), @r###"
    <timestamp> DEBG receiving probe request
    <timestamp> TRCE received external request: Probe(None)
    <timestamp> INFO update received: 1.2 (87effe73b80453f397cee4db3c3589a8630b220876dff8fb23447315037ff96d)
    <timestamp> DEBG updating last polling time
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> TRCE starting to handle: validation
    <timestamp> INFO no signature key available on device, ignoring signature validation
    <timestamp> TRCE starting to handle: download
    <timestamp> TRCE the following objects are missing: [("testfile", "23c3c412177bd37b9b61bf4738b18dc1fe003811c2583a14d2d9952d8b6a75b4")]
    <timestamp> DEBG starting download of: testfile (23c3c412177bd37b9b61bf4738b18dc1fe003811c2583a14d2d9952d8b6a75b4)
    <timestamp> TRCE starting to handle: error
    <timestamp> ERRO error state reached: Permission denied (os error 13)
    <timestamp> INFO returning to machine's entry point
    <timestamp> TRCE starting to handle: entry_point
    <timestamp> DEBG polling is disabled
    <timestamp> TRCE starting to handle: park
    <timestamp> INFO parking state machine
    "###);

    insta::assert_snapshot!(remove_carriage_newline_characters(output_client), @r###"
    Update available. The update is running in background.
    "###);

    insta::assert_snapshot!(rewrite_log_output(output_log).0, @r###"
    <timestamp> INFO update received: 1.2 (87effe73b80453f397cee4db3c3589a8630b220876dff8fb23447315037ff96d)
    <timestamp> DEBG updating last polling time
    <timestamp> DEBG saved runtime settings to "<file>"
    <timestamp> TRCE starting to handle: validation
    <timestamp> INFO no signature key available on device, ignoring signature validation
    <timestamp> TRCE starting to handle: download
    <timestamp> TRCE the following objects are missing: [("testfile", "23c3c412177bd37b9b61bf4738b18dc1fe003811c2583a14d2d9952d8b6a75b4")]
    <timestamp> DEBG starting download of: testfile (23c3c412177bd37b9b61bf4738b18dc1fe003811c2583a14d2d9952d8b6a75b4)
    <timestamp> TRCE starting to handle: error
    <timestamp> ERRO error state reached: Permission denied (os error 13)
    <timestamp> INFO returning to machine's entry point
    <timestamp> TRCE starting to handle: entry_point
    <timestamp> DEBG polling is disabled
    <timestamp> TRCE starting to handle: park
    <timestamp> INFO parking state machine
    "###);
}

#[test]
#[cfg(not(feature = "v1-parsing"))]
fn failing_invalid_file_config() {
    use std::io::Write;

    let setup = Settings::default();
    let mut file = tempfile::NamedTempFile::new().unwrap();
    let file_path = file.path().to_owned();

    write!(
        file,
        r#"[network]
    server_address=https://api.updatehub.io, listen_socket=localhost:8080;"#
    )
    .unwrap();

    let (mut session, _setup) = setup.config_file(file_path).init_server();
    let output_server = session.exp_eof().unwrap();
    let (output_server_trce, ..) = rewrite_log_output(output_server);

    insta::assert_snapshot!(output_server_trce, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> DEBG loading system settings from "<file>"
    unexpected character found: `/` at line 2 column 26
    "###);
}

#[test]
#[cfg(feature = "v1-parsing")]
fn failing_invalid_file_config() {
    use std::io::Write;

    let setup = Settings::default();
    let mut file = tempfile::NamedTempFile::new().unwrap();
    let file_path = file.path().to_owned();

    write!(
        file,
        r#"[network]
    server_address=https://api.updatehub.io, listen_socket=localhost:8080;"#
    )
    .unwrap();

    let (mut session, _setup) = setup.config_file(file_path).init_server();
    let output_server = session.exp_eof().unwrap();
    let (output_server_trce, ..) = rewrite_log_output(output_server);

    insta::assert_snapshot!(output_server_trce, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> DEBG loading system settings from "<file>"
    Parsing error: toml: unexpected character found: `/` at line 2 column 26, ini: Custom("missing field `Network`")
    "###);
}

#[test]
fn failing_invalid_server_address() {
    let setup = Settings::default();
    let (mut session, setup) = setup.timeout(300).init_server();
    let _mocks = create_mock_server(FakeServer::NoUpdate);
    let output_server_1 = get_output_server(&mut session, StopMessage::Polling(Polling::Disable));

    let output_client = run_client_probe(
        Server::Custom("http://foo:--".to_string()),
        &setup.settings.data.network.listen_socket,
    );
    let output_server_2 = get_output_server(
        &mut session,
        StopMessage::Custom(
            r#"\r\n.* TRCE received external request: Probe\(Some\("http://foo:--"\)\).*"#
                .to_string(),
        ),
    );
    let output_log = run_client_log(&setup.settings.data.network.listen_socket);

    let (output_server_trce_1, output_server_info_1) = rewrite_log_output(output_server_1);
    let (output_server_trce_2, ..) = rewrite_log_output(output_server_2);

    insta::assert_snapshot!(output_server_info_1, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
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

    insta::assert_snapshot!(output_server_trce_2.trim(), @r###"
    <timestamp> DEBG receiving probe request
    "###);

    insta::assert_snapshot!(remove_carriage_newline_characters(output_client), @r###"
    Unexpected response: InternalServerError
    "###);

    insta::assert_snapshot!(rewrite_log_output(output_log).0, @"<timestamp> DEBG receiving log request
");
}

#[test]
fn failing_fail_check_requirements() {
    let setup = Settings::default();

    let (mut session, setup) = setup.timeout(300).init_server();
    let _mocks = create_mock_server(FakeServer::CheckRequirementsTest(
        setup.firmware.data.product_uid.clone(),
    ));
    let output_server_1 = get_output_server(&mut session, StopMessage::Polling(Polling::Disable));

    let output_client =
        run_client_probe(Server::Standard, &setup.settings.data.network.listen_socket);
    let output_server_2 = get_output_server(&mut session, StopMessage::Polling(Polling::Disable));
    let output_log = run_client_log(&setup.settings.data.network.listen_socket);

    let (output_server_trce_1, output_server_info_1) = rewrite_log_output(output_server_1);
    let (output_server_trce_2, output_server_info_2) = rewrite_log_output(output_server_2);

    insta::assert_snapshot!(output_server_info_1, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
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

    insta::assert_snapshot!(output_server_info_2.trim(), @r###"
    <timestamp> INFO update received: 1.2 (fb21b217cb83e8af368c773eb13bad0a94e1b0088c6bf561072decf3c1ae9df3)
    <timestamp> INFO no signature key available on device, ignoring signature validation
    <timestamp> INFO installing update: 1.2 (fb21b217cb83e8af368c773eb13bad0a94e1b0088c6bf561072decf3c1ae9df3)
    <timestamp> INFO using installation set as target 1
    <timestamp> ERRO error state reached: fail to check the requirements
    <timestamp> INFO returning to machine's entry point
    <timestamp> INFO parking state machine
    "###);

    insta::assert_snapshot!(output_server_info_2.trim(), @r###"
    <timestamp> INFO update received: 1.2 (fb21b217cb83e8af368c773eb13bad0a94e1b0088c6bf561072decf3c1ae9df3)
    <timestamp> INFO no signature key available on device, ignoring signature validation
    <timestamp> INFO installing update: 1.2 (fb21b217cb83e8af368c773eb13bad0a94e1b0088c6bf561072decf3c1ae9df3)
    <timestamp> INFO using installation set as target 1
    <timestamp> ERRO error state reached: fail to check the requirements
    <timestamp> INFO returning to machine's entry point
    <timestamp> INFO parking state machine
    "###);

    insta::assert_snapshot!(output_server_trce_2.trim(), @r###"
    <timestamp> DEBG receiving probe request
    <timestamp> TRCE received external request: Probe(None)
    <timestamp> INFO update received: 1.2 (fb21b217cb83e8af368c773eb13bad0a94e1b0088c6bf561072decf3c1ae9df3)
    <timestamp> DEBG updating last polling time
    <timestamp> DEBG saved runtime settings to "<file>"
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
    <timestamp> INFO installing update: 1.2 (fb21b217cb83e8af368c773eb13bad0a94e1b0088c6bf561072decf3c1ae9df3)
    <timestamp> INFO using installation set as target 1
    <timestamp> TRCE starting to handle: error
    <timestamp> ERRO error state reached: fail to check the requirements
    <timestamp> INFO returning to machine's entry point
    <timestamp> TRCE starting to handle: entry_point
    <timestamp> DEBG polling is disabled
    <timestamp> TRCE starting to handle: park
    <timestamp> INFO parking state machine
    "###);

    insta::assert_snapshot!(remove_carriage_newline_characters(output_client), @r###"
    Update available. The update is running in background.
    "###);

    insta::assert_snapshot!(rewrite_log_output(output_log).0, @r###"
    <timestamp> INFO update received: 1.2 (fb21b217cb83e8af368c773eb13bad0a94e1b0088c6bf561072decf3c1ae9df3)
    <timestamp> DEBG updating last polling time
    <timestamp> DEBG saved runtime settings to "<file>"
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
    <timestamp> INFO installing update: 1.2 (fb21b217cb83e8af368c773eb13bad0a94e1b0088c6bf561072decf3c1ae9df3)
    <timestamp> INFO using installation set as target 1
    <timestamp> TRCE starting to handle: error
    <timestamp> ERRO error state reached: fail to check the requirements
    <timestamp> INFO returning to machine's entry point
    <timestamp> TRCE starting to handle: entry_point
    <timestamp> DEBG polling is disabled
    <timestamp> TRCE starting to handle: park
    <timestamp> INFO parking state machine
    "###);
}
