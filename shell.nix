{ pkgs ? import <nixpkgs> {} }:

with pkgs;

stdenv.mkDerivation {
  name = "updatehub";
  buildInputs = [
    pkg-config
    openssl
    libarchive
  ];
}
