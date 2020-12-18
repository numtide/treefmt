{ rustChannelOf }:
(rustChannelOf {
  rustToolchain = ./rust-toolchain;
  sha256 = "sha256-bTH8PbGZvRa5PNPTTI546BW2XXasUC4mntSpLHVU15o=";
}).rust.override {
  extensions = [
    "clippy-preview"
    "rls-preview"
    "rustfmt-preview"
    "rust-analysis"
    "rust-std"
    "rust-src"
  ];
  targets = [
    "wasm32-unknown-unknown"
    "x86_64-unknown-linux-gnu"
  ];
}
