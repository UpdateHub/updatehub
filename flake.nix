{
  description = "UpdateHub Development Environment";

  inputs = {
    nixpkgs.url = "nixpkgs/nixos-unstable";
    flake-utils = {
      url = "github:numtide/flake-utils";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
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
          ];

          shellHook = with pkgs; ''
            # loopdev 0.3.0 uses bindgen to generate its bindings
            export LIBCLANG_PATH="${llvmPackages.libclang.lib}/lib"
            export BINDGEN_EXTRA_CLANG_ARGS="-I${linuxHeaders}/include/"
          '';
        };
      });
}
