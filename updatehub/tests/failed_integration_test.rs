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
    <timestamp> TRCE moving to Download state to process the update package.
    <timestamp> ERRO error state reached: Permission denied (os error 13)
    <timestamp> INFO returning to machine's entry point
    <timestamp> DEBG polling is disabled, parking the state machine.
    <timestamp> DEBG staying on Park state.
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
    let (output_server_trce, ..) = format_output_server(output_server);

    insta::assert_snapshot!(output_server_trce, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> DEBG loading system settings from "<file>"...
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
    let (output_server_trce, ..) = format_output_server(output_server);

    insta::assert_snapshot!(output_server_trce, @r###"
    <timestamp> INFO starting UpdateHub Agent <version>
    <timestamp> DEBG loading system settings from "<file>"...
    Custom("missing field `Network`")
    "###);
}

#[test]
fn failing_invalid_server_address() {
    let setup = Settings::default();
    let (mut session, _setup) = setup.timeout(300).init_server();
    let _mocks = create_mock_server(FakeServer::NoUpdate);
    let output_server_1 = get_output_server(&mut session, StopMessage::Polling(Polling::Disable));

    let output_client = run_client_probe(Server::Custom("http://foo:--".to_string()));
    let output_server_2 = get_output_server(
        &mut session,
        StopMessage::Custom(
            "\r\n.* TRCE Received external request: Probe\\(Some\\(\"http://foo:--\"\\)\\).*"
                .to_string(),
        ),
    );
    let output_log = run_client_log();

    let (output_server_trce_1, output_server_info_1) = format_output_server(output_server_1);
    let (output_server_trce_2, ..) = format_output_server(output_server_2);

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
    "###);

    insta::assert_snapshot!(remove_carriage_newline_characters(output_client), @r###"
    Err(
        UnexpectedResponse(
            InternalServerError,
        ),
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
}

#[test]
fn failing_fail_check_requirements() {
    let setup = Settings::default();

    let (mut session, setup) = setup.timeout(300).init_server();
    let _mocks = create_mock_server(FakeServer::CheckRequirementsTest(
        setup.firmware.data.product_uid.clone(),
    ));
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

    insta::assert_snapshot!(output_server_info_2.trim(), @r###"
    <timestamp> INFO installing update: fb21b217cb83e8af368c773eb13bad0a94e1b0088c6bf561072decf3c1ae9df3
    <timestamp> INFO using installation set as target 1
    <timestamp> ERRO error state reached: fail to check the requirements
    <timestamp> INFO returning to machine's entry point
    "###);

    insta::assert_snapshot!(output_server_info_2.trim(), @r###"
    <timestamp> INFO installing update: fb21b217cb83e8af368c773eb13bad0a94e1b0088c6bf561072decf3c1ae9df3
    <timestamp> INFO using installation set as target 1
    <timestamp> ERRO error state reached: fail to check the requirements
    <timestamp> INFO returning to machine's entry point
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
    <timestamp> INFO installing update: fb21b217cb83e8af368c773eb13bad0a94e1b0088c6bf561072decf3c1ae9df3
    <timestamp> INFO using installation set as target 1
    <timestamp> ERRO error state reached: fail to check the requirements
    <timestamp> INFO returning to machine's entry point
    <timestamp> DEBG polling is disabled, parking the state machine.
    <timestamp> DEBG staying on Park state.
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
