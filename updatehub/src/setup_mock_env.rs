// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

#[tokio::main]
async fn main() {
    let setup = updatehub::tests::TestEnvironment::build()
        .server_address(mockito::server_url())
        .disable_polling()
        .add_echo_binary("reboot")
        .finish();
    let _mocks = start_mock(&setup.firmware.data.product_uid);

    println!(
        r#"PATH="{bin_dir}:$PATH" cargo run --bin updatehub daemon -v trace -c {conf_file}"#,
        bin_dir = setup.binaries.stored_path.to_string_lossy(),
        conf_file = setup.settings.stored_path.to_string_lossy()
    );
    println!("Mock running in background");

    async_ctrlc::CtrlC::new().unwrap().await;
    println!("Done!");
}

fn start_mock(product_uid: &str) -> Vec<mockito::Mock> {
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

    vec![
            mockito::mock("POST", "/upgrades")
                .match_header("Content-Type", "application/json")
                .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
                .with_status(200)
                .with_body(package_metadata.to_string())
                .create(),
            mockito::mock(
                "GET",
                format!("/products/{}/packages/96ac17e535dba0cc5e4aed4ccc4f7deaabd8e714955cbc93a79fe618b6b66ca8/objects/23c3c412177bd37b9b61bf4738b18dc1fe003811c2583a14d2d9952d8b6a75b4", product_uid)
                    .as_str(),
            )
                .match_header("Content-Type", "application/json")
                .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
                .with_status(200)
                .with_body(std::iter::repeat(0xF).take(40960).collect::<Vec<_>>())
                .create(),
    ]
}
