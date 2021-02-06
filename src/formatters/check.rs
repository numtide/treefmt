use crate::formatters::manifest::RootManifest;
use crate::formatters::tool::CmdContext;
use crate::CLOG;
use anyhow::{Error, Result};
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
        .map(|(new, old)| {
            Ok(CmdContext {
                command: new.command.clone(),
                options: new.options.clone(),
                metadata: if new.command != old.command || new.options != old.options {
                    // If either the command or the options have changed, invalidate old entries
                    new.metadata.clone()
                } else {
                    new.metadata.difference(&old.metadata).cloned().collect()
                },
            })
        })
        .collect::<Result<Vec<CmdContext>, Error>>()?;

    if cache_context.iter().all(|f| f.metadata.is_empty()) {
        CLOG.debug(&format!("No changes found in {}", prjfmt_toml.display()));
        return Ok(Vec::new());
    }

    CLOG.info(&format!("The following file has changed or newly added:"));
    for cmd in &cache_context {
        if !cmd.metadata.is_empty() {
            for p in &cmd.metadata {
                CLOG.info(&format!(
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

#[cfg(test)]
mod tests {
    use super::*;
    use std::collections::BTreeMap;

    /// Every same path produce same hash
    #[test]
    fn test_check_prjfmt() -> Result<()> {
        let prjfmt_path = PathBuf::from(r"examples/monorepo/prjfmt.toml");
        let cache: RootManifest = RootManifest {
            manifest: BTreeMap::new(),
        };
        let cmd_context: Vec<CmdContext> = Vec::new();

        assert_eq!(
            check_prjfmt(&prjfmt_path, &cmd_context, &cache)?,
            cmd_context
        );
        Ok(())
    }
}
