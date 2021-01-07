use crate::emoji;
use console::style;

use anyhow::{anyhow, Result};
use serde::{de::Error, Deserialize, Deserializer};
use std::convert::TryFrom;
use std::fmt;
use tools::{
    haskell::{Hfmt, deserialize_haskell},
    go::{Gfmt, deserialize_go},
    rust::{Rfmt, deserialize_rust}
};

use super::tools;

/// Represents the set of formatter tools `allfmt` uses
#[derive(Debug, Deserialize)]
#[serde(untagged, rename_all = "lowercase")]
pub enum Tools {
    /// Haskell formatter tool
    #[serde(deserialize_with = "deserialize_haskell")]
    HaskellFmts(Hfmt),
    /// gofmt tools
    #[serde(deserialize_with = "deserialize_go")]
    GoFmts(Gfmt),
    /// rustfmt tools
    #[serde(deserialize_with = "deserialize_rust")]
    RustFmts(Rfmt),
}
