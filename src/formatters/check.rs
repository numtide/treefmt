use super::tool::FileMeta;
use crate::formatters::manifest::RootManifest;
use crate::formatters::tool::{create_command_context, CmdContext};
use crate::CLOG;
use anyhow::{anyhow, Error, Result};
use std::collections::BTreeSet;
use std::path::PathBuf;
use std::vec::Vec;

/// Checking content of cache's file and current prjfmt runs
pub fn check_prjfmt(
    prjfmt_toml: &PathBuf,
    cmd_context: &Vec<CmdContext>,
    cache: &RootManifest,
) -> Result<Vec<CmdContext>> {
    let cache_context = cache.manifest.values().map(|b| b).into_iter();
    let results = cmd_context.into_iter().zip(cache_context);

    let cache_context: Vec<CmdContext> = results
        .clone()
        .map(|(a, b)| {
            Ok(CmdContext {
                command: a.command.clone(),
                args: a.args.clone(),
                metadata: a.metadata.difference(&b.metadata).cloned().collect(),
            })
        })
        .collect::<Result<Vec<CmdContext>, Error>>()?;

    if cache_context.iter().all(|f| f.metadata.is_empty()) {
        CLOG.warn(&format!("No changes found in {}", prjfmt_toml.display()));
        return Ok(Vec::new());
    }

    CLOG.warn(&format!("The following file has changed or newly added:"));
    for cmd in &cache_context {
        if !cmd.metadata.is_empty() {
            for p in &cmd.metadata {
                CLOG.warn(&format!(
                    " - {} last modification time: {}",
                    p.path.display(),
                    p.mtimes
                ));
            }
        }
    }
    // return Err(anyhow!("prjfmt failed to run."));
    Ok(cache_context)
}
