use super::tool::FileMeta;
use crate::formatters::manifest::RootManifest;
use crate::formatters::tool::create_command_context;
use crate::CLOG;
use anyhow::{anyhow, Error, Result};
use std::collections::BTreeSet;
use std::path::PathBuf;
use std::vec::Vec;

/// Checking content of cache's file and current prjfmt runs
pub fn check_prjfmt(prjfmt_toml: PathBuf, cache: &RootManifest) -> Result<()> {
    let cmd_context = create_command_context(&prjfmt_toml)?
        .into_iter()
        .map(|a| a.metadata);
    let cache_context = cache
        .manifest
        .values()
        .map(|b| Ok(&b.metadata))
        .collect::<Result<Vec<&BTreeSet<FileMeta>>, Error>>()?;

    let results: Vec<(BTreeSet<FileMeta>, &BTreeSet<FileMeta>)> =
        cmd_context.zip(cache_context).collect();

    let mut new_vec: Vec<FileMeta> = Vec::new();

    for (a, b) in results {
        let mut match_meta: Vec<FileMeta> = a.difference(b).cloned().collect();
        if let false = match_meta.is_empty() {
            new_vec.append(&mut match_meta)
        }
    }

    if let false = new_vec.is_empty() {
        CLOG.warn(&format!("The following file has changed or newly added:"));
        for p in new_vec {
            CLOG.warn(&format!(
                " - {} last modification time: {}",
                p.path.display(),
                p.mtimes
            ));
        }
        return Err(anyhow!("prjfmt failed to run."));
    }

    Ok(())
}
