use super::RootManifest;
use crate::{customlog, CmdContext, CLOG};

use anyhow::{anyhow, Result};
use hex;
use sha1::{Digest, Sha1};
use std::collections::BTreeMap;
use std::fs::{read_to_string, File};
use std::io::Write;
use std::path::PathBuf;

/// Create <hex(hash(path-to-treefmt))>.toml and put it in $XDG_CACHE_DIR/treefmt/eval-cache/
pub fn create_manifest(
    treefmt_toml: PathBuf,
    cache_dir: PathBuf,
    cmd_ctx: Vec<CmdContext>,
) -> Result<()> {
    let hash_toml = create_hash(&treefmt_toml)?;

    let mut f = File::create(cache_dir.join(hash_toml))?;
    let map_manifest: BTreeMap<String, CmdContext> = cmd_ctx
        .into_iter()
        .map(|cmd| {
            let treefmt = cmd.command;
            let manifest = CmdContext {
                command: treefmt.to_string(),
                options: cmd.options,
                metadata: cmd.metadata,
            };
            (treefmt.to_string(), manifest)
        })
        .collect();
    let manifest_toml = RootManifest {
        manifest: map_manifest,
    };
    f.write_all(
        format!(
            "# {} DO NOT HAND EDIT THIS FILE {}\n\n{}",
            customlog::WARN,
            customlog::WARN,
            toml::to_string_pretty(&manifest_toml)?
        )
        .as_bytes(),
    )?;
    Ok(())
}

/// Read the <hex(hash(path-to-treefmt))>.toml and return list of config's cache evaluation
pub fn read_manifest(treefmt_toml: &PathBuf, cache_dir: &PathBuf) -> Result<RootManifest> {
    let hash_toml = create_hash(&treefmt_toml)?;
    let manifest_toml = cache_dir.join(&hash_toml);

    if manifest_toml.exists() {
        CLOG.debug(&format!("Found {} in: {}", hash_toml, cache_dir.display()));
        let open_file = match read_to_string(&manifest_toml) {
            Ok(file) => file,
            Err(err) => {
                return Err(anyhow!(
                    "Cannot open {} due to {}.",
                    manifest_toml.display(),
                    err
                ))
            }
        };

        let manifest_content: RootManifest = toml::from_str(&open_file)?;
        Ok(manifest_content)
    } else {
        CLOG.debug(&format!("{} not found!", hash_toml));
        Ok(RootManifest {
            manifest: BTreeMap::new(),
        })
    }
}

fn create_hash(treefmt_toml: &PathBuf) -> Result<String> {
    let treefmt_str = match treefmt_toml.to_str() {
        Some(str) => str.as_bytes(),
        None => {
            return Err(anyhow!(
                "{}cannot convert to string slice",
                customlog::ERROR
            ))
        }
    };
    let treefmt_hash = Sha1::digest(treefmt_str);
    let result = hex::encode(treefmt_hash);
    let manifest_toml = format!("{}.toml", result);
    Ok(manifest_toml)
}

#[cfg(test)]
mod tests {
    use super::*;

    /// Every same path produce same hash
    #[test]
    fn test_create_hash() -> Result<()> {
        let file_path = PathBuf::from(r"examples/monorepo/treefmt.toml");
        let treefmt_hash = "3076c82c47ced65a86ffe285ac9d941d812bfdad.toml";
        assert_eq!(create_hash(&file_path)?, treefmt_hash);
        Ok(())
    }
}
