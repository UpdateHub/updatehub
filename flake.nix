{
  description = "UpdateHub Development Environment";

  inputs = {
    nixpkgs.url = "nixpkgs/release-22.11";
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

        rust-toolchain = with rust.packages.${system};
          combine [
            (stable.withComponents [ "rustc" "cargo" "rust-src" "clippy" ])
            (latest.withComponents [ "rustfmt" "rust-analyzer" ])
          ];
      in
      {
        devShell = pkgs.mkShell {
          buildInputs = with pkgs; [
            libarchive
            linuxHeaders
            llvmPackages.libclang
            openssl
            pkg-config
            protobuf

            cargo-insta
            cargo-limit
            cargo-outdated
            cargo-release
            cargo-watch
            rust-toolchain

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
