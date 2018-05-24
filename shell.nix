let
  pkgs = import <nixpkgs> {};
in pkgs.callPackage ./default.nix {}
