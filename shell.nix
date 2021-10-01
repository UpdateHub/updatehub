{ pkgs ? import <nixpkgs> {} }:

with pkgs;

stdenv.mkDerivation {
  name = "updatehub";
  buildInputs = [
    libarchive
    linuxHeaders
    llvmPackages.libclang
    openssl
    pkg-config
    protobuf
  ];

  # loopdev 0.3.0 uses bindgen to generate its bindings
  LIBCLANG_PATH = "${llvmPackages.libclang.lib}/lib";
  BINDGEN_EXTRA_CLANG_ARGS = "-I${linuxHeaders}/include/";
}
