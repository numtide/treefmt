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
    cache: &RootManifest,
) -> Result<(Vec<CmdContext>, Option<Vec<CmdContext>>)> {
    let cmd_context = create_command_context(&prjfmt_toml)?;
    let cache_context = cache.manifest.values().map(|b| b).into_iter();
    let results = cmd_context.clone().into_iter().zip(cache_context);

    let cache_context: Vec<CmdContext> = results
        .clone()
        .map(|(a, b)| {
            if a.command != b.command {
                CLOG.warn(&format!(
                    "Command has changed! Please remove .prjfmt folder"
                ));
                return Err(anyhow!("prjfmt failed to run."));
            }
            if a.args.len() != b.args.len() {
                CLOG.warn(&format!(
                    "Arguments has changed! Please remove .prjfmt folder"
                ));
                return Err(anyhow!("prjfmt failed to run."));
            }
            Ok(CmdContext {
                command: a.command,
                args: a.args,
                metadata: a.metadata.difference(&b.metadata).cloned().collect(),
            })
        })
        .collect::<Result<Vec<CmdContext>, Error>>()?;

    if cache_context.iter().all(|f| f.metadata.is_empty()) {
        CLOG.warn(&format!("No changes found in {}", prjfmt_toml.display()));
        return Ok((cmd_context, None));
    }

    let updated_context = results
        .map(|(a, b)| {
            if a.command != b.command {
                CLOG.warn(&format!(
                    "Command has changed! Please remove .prjfmt folder"
                ));
                return Err(anyhow!("prjfmt failed to run."));
            }
            if a.args.len() != b.args.len() {
                CLOG.warn(&format!(
                    "Arguments has changed! Please remove .prjfmt folder"
                ));
                return Err(anyhow!("prjfmt failed to run."));
            }
            Ok(CmdContext {
                command: a.command,
                args: a.args,
                metadata: a.metadata.union(&b.metadata).cloned().collect(),
            })
        })
        .collect::<Result<Vec<CmdContext>, Error>>()?;

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
    Ok((cache_context, Some(updated_context)))
}
