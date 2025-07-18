// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

#[tokio::main]
async fn main() {
    let server = mockito::Server::new();

    let setup = updatehub::tests::TestEnvironment::build()
        .server_address(server.url())
        .disable_polling()
        .add_echo_binary("reboot")
        .finish();
    let _mocks = start_mock(server, &setup.firmware.data.product_uid);

    println!(
        r#"PATH="{bin_dir}:$PATH" cargo run --bin updatehub daemon -v trace -c {conf_file}"#,
        bin_dir = setup.binaries.stored_path.to_string_lossy(),
        conf_file = setup.settings.stored_path.to_string_lossy()
    );
    println!("Mock running in background");

    async_ctrlc::CtrlC::new().unwrap().await;
    println!("Done!");
}

fn start_mock(mut server: mockito::ServerGuard, product_uid: &str) -> mockito::Mock {
    use serde_json::json;

    let package_metadata = json!({
        "product": "0123456789",
        "version": "1.2",
        "supported-hardware": ["board"],
        "objects": [
            [
                {
                    "mode": "test",
                    "filename": "testfile",
                    "target": "/dev/device1",
                    "sha256sum": "23c3c412177bd37b9b61bf4738b18dc1fe003811c2583a14d2d9952d8b6a75b4",
                    "size": 40960,
                    "force-check-requirements-fail": false
                }
            ],
            [
                {
                    "mode": "test",
                    "filename": "testfile",
                    "target": "/dev/device1",
                    "sha256sum": "23c3c412177bd37b9b61bf4738b18dc1fe003811c2583a14d2d9952d8b6a75b4",
                    "size": 40960,
                    "force-check-requirements-fail": false
                }
            ]
        ]
    });

    server
        .mock("POST", "/upgrades")
        .match_header("Content-Type", "application/json")
        .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
        .with_status(200)
        .with_body(package_metadata.to_string())
        .create();

    server.mock(
        "GET",
        format!("/products/{product_uid}/packages/96ac17e535dba0cc5e4aed4ccc4f7deaabd8e714955cbc93a79fe618b6b66ca8/objects/23c3c412177bd37b9b61bf4738b18dc1fe003811c2583a14d2d9952d8b6a75b4").as_str()
        )
        .match_header("Content-Type", "application/json")
        .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
        .with_status(200)
        .with_body(std::iter::repeat_n(0xF, 40960).collect::<Vec<_>>())
            .create()
}
