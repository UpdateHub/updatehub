{
  description = "UpdateHub Development Environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-26.05";
    flake-utils.url = "github:numtide/flake-utils";

    rust = {
      url = "github:nix-community/fenix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = { self, nixpkgs, flake-utils, rust }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};

        # The MSRV. CI builds and tests with this exact toolchain, so bumping
        # it here is what bumps the minimum supported Rust version.
        rust-toolchain = with rust.packages.${system};
          let
            msrv = toolchainOf {
              channel = "1.82.0";
              sha256 = "sha256-yMuSb5eQPO/bHv+Bcf/US8LVMbf/G/0MSfiPwBhiPpk=";
            };
          in
          combine [
            (msrv.withComponents [ "rustc" "cargo" "rust-src" "clippy" "llvm-tools" ])
            # rustfmt.toml uses nightly-only options, so rustfmt comes from
            # nightly regardless of the MSRV used to build.
            (latest.withComponents [ "rustfmt" "rust-analyzer" ])
          ];
      in
      {
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            libarchive
            linuxHeaders
            llvmPackages.libclang
            openssl
            pkg-config
            protobuf

            cargo-insta
            cargo-limit
            cargo-llvm-cov
            cargo-outdated
            cargo-release
            cargo-watch
            rust-toolchain

            # used by the listener example test
            socat

            # used by excluded tests
            mtdutils
          ];

          shellHook = with pkgs; ''
            # loopdev 0.3.0 uses bindgen to generate its bindings
            export LIBCLANG_PATH="${llvmPackages.libclang.lib}/lib"
            export BINDGEN_EXTRA_CLANG_ARGS="-I${linuxHeaders}/include/"
          '';
        };
      });
}
