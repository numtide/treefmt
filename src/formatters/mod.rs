//! Functionality related to installing prebuilt binaries

#![deny(missing_docs)]
/// File checking utility
pub mod check;
/// Manifest configuration
pub mod manifest;

use crate::CmdContext;
use serde::{Deserialize, Serialize};
use std::collections::BTreeMap;

#[derive(Debug, Deserialize, Serialize)]
/// RootManifest
pub struct RootManifest {
    /// Map of manifests config based on its formatter
    pub manifest: BTreeMap<String, CmdContext>,
}
