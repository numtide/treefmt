#![deny(missing_docs)]

use serde::{de::Error, Deserialize, Deserializer};
use crate::emoji;
use console::style;

/// Cargo const
pub const RUST_FMT_CARGO: &str = "cargo";
/// Rustfmt const
pub const RUST_FMT_RUSTFMT: &str = "rustfmt";

/// Name of Rust formatter
#[derive(Debug, Deserialize)]
pub enum Rfmt {
    /// cargo fmt command
    Cargo,
    ///rustfmt command
    RustFmt
}

/// Parse config's input into data type
pub fn from_config<'de, D>(code: &str) -> std::result::Result<Rfmt, D::Error>
    where
        D: Deserializer<'de>,
    {
        match code.as_ref() {
            RUST_FMT_CARGO => Ok(Rfmt::Cargo),
            RUST_FMT_RUSTFMT => Ok(Rfmt::RustFmt),
            _ => Err(D::Error::custom(format!(
                "{} {}",
                emoji::ERROR,
                style("Unknown Haskell formatter").bold().red(),
            ))),
        }
    }

/// Implementation of deserialization for Rust
pub fn deserialize_rust<'de, D>(deserializer: D) -> std::result::Result<Rfmt, D::Error>
where
    D: Deserializer<'de>,
{
    let s: String = Deserialize::deserialize(deserializer)?;
    from_config::<D>(&s)
}