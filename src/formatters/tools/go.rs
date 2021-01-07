use serde::{de::Error, Deserialize, Deserializer};
use crate::emoji;
use console::style;

/// Gofmt const
pub const GO_FMT_GOFMT: &str = "gofmt";

/// Name of Go formatter
#[derive(Debug, Deserialize)]
pub enum Gfmt {
    /// the default gofmt
    GoFmt
}

/// Parse config's input into data type
pub fn from_config<'de, D>(code: &str) -> std::result::Result<Gfmt, D::Error>
    where
        D: Deserializer<'de>,
    {
        match code.as_ref() {
            GO_FMT_GOFMT=> Ok(Gfmt::GoFmt),
            _ => Err(D::Error::custom(format!(
                "{} {}",
                emoji::ERROR,
                style("Unknown Go formatter").bold().red(),
            ))),
        }
    }

/// Implementation of deserialization for Go
pub fn deserialize_go<'de, D>(deserializer: D) -> std::result::Result<Gfmt, D::Error>
where
    D: Deserializer<'de>,
{
    let s: String = Deserialize::deserialize(deserializer)?;
    from_config::<D>(&s)
}