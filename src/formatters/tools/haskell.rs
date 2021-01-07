use serde::{de::Error, Deserialize, Deserializer};
use crate::emoji;
use console::style;

/// Brittany const
pub const HASKELL_FMT_BRITTANY: &str = "brittany";
/// Floskell const
pub const HASKELL_FMT_FLOSKELL: &str = "floskell";
/// Fourmolu const
pub const HASKELL_FMT_FOURMOLU: &str = "fourmolu";
/// Ormolu const
pub const HASKELL_FMT_ORMOLU: &str = "ormolu";
/// Stylish-Haskell const
pub const HASKELL_FMT_STYLISHHASKELL: &str = "stylish-haskell";

/// List of Haskell formatters that are available
#[derive(Debug, Deserialize)]
pub enum Hfmt {
    /// Brittany
    Brittany,
    /// Floskell
    Floskell,
    /// Fourmolu
    Fourmolu,
    /// Ormolu (default)
    Ormolu,
    /// Stylish-Haskell
    StylishHaskell,
}

/// Parse config's input into data type
pub fn from_config<'de, D>(code: &str) -> std::result::Result<Hfmt, D::Error>
    where
        D: Deserializer<'de>,
    {
        match code.as_ref() {
            HASKELL_FMT_BRITTANY => Ok(Hfmt::Brittany),
            HASKELL_FMT_FLOSKELL => Ok(Hfmt::Floskell),
            HASKELL_FMT_FOURMOLU => Ok(Hfmt::Fourmolu),
            HASKELL_FMT_ORMOLU => Ok(Hfmt::Ormolu),
            HASKELL_FMT_STYLISHHASKELL => Ok(Hfmt::StylishHaskell),
            _ => Err(D::Error::custom(format!(
                "{} {}",
                emoji::ERROR,
                style("Unknown Haskell formatter").bold().red(),
            ))),
        }
    }

/// Implementation of deserialization for Haskell
pub fn deserialize_haskell<'de, D>(deserializer: D) -> std::result::Result<Hfmt, D::Error>
where
    D: Deserializer<'de>,
{
    let s: String = Deserialize::deserialize(deserializer)?;
    from_config::<D>(&s)
}