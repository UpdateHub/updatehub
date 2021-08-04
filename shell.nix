{ pkgs ? import <nixpkgs> {} }:

with pkgs;

stdenv.mkDerivation {
  name = "updatehub";
  buildInputs = [
    pkg-config
    openssl
    libarchive
    llvmPackages.libclang
    linuxHeaders
  ];

  # loopdev 0.3.0 uses bindgen to generate its bindings
  LIBCLANG_PATH = "${llvmPackages.libclang.lib}/lib";
  BINDGEN_EXTRA_CLANG_ARGS = "-I${linuxHeaders}/include/";
}
