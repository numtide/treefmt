use super::manifest::create_prjfmt_manifest;
use crate::{emoji, CLOG};
use anyhow::{anyhow, Error, Result};
use filetime::FileTime;
use glob;
use serde::{Deserialize, Serialize};
use std::cmp::Ordering;
use std::collections::{BTreeMap, BTreeSet};
use std::fs::{metadata, read_to_string};
use std::iter::Iterator;
use std::path::PathBuf;
use xshell::cmd;

/// Make sure that formatter binary exists. This also for other formatter
pub fn check_bin(command: &str) -> Result<()> {
    let cmd_bin = command.split_ascii_whitespace().next().unwrap_or("");
    if let Ok(str) = cmd!("which {cmd_bin}").read() {
        CLOG.info(&format!("Found {} at {}", cmd_bin, str));
        return Ok(());
    }
    anyhow::bail!(
        "Failed to locate formatter named {}. \
        Please make sure it is available in your PATH",
        command
    )
}

/// Running the prjfmt
// TODO: make run_prjfmt only takes 1 argument &FmtConfig
pub fn run_prjfmt(cwd: PathBuf, cache_dir: PathBuf) -> anyhow::Result<()> {
    let prjfmt_toml = cwd.as_path().join("prjfmt.toml");
    if let false = prjfmt_toml.as_path().exists() {
        return Err(anyhow!(
            "{}prjfmt.toml not found, please run --init command",
            emoji::ERROR
        ));
    }

    let cmd_contexts = create_command_context(&prjfmt_toml)?;

    for ctx in &cmd_contexts {
        check_bin(&ctx.command)?;
    }

    println!("===========================");
    for ctx in &cmd_contexts {
        println!("Command: {}", ctx.command);
        println!("Files:");
        for m in &ctx.metadata {
            let path = &m.path;
            println!(" - {}", path.display());
        }
        println!("===========================");
    }

    for ctx in &cmd_contexts {
        for m in &ctx.metadata {
            let arg = &ctx.args;
            let cmd_arg = &ctx.command;
            let path = &m.path;
            cmd!("{cmd_arg} {arg...} {path}").run()?;
        }
    }

    println!("Format successful");
    println!("capturing formatted file's state...");
    create_prjfmt_manifest(prjfmt_toml, cache_dir, cmd_contexts)?;

    Ok(())
}

/// Convert glob pattern into list of pathBuf
/// TODO: Implement includes and exclude filter
pub fn glob_to_path(
    cwd: &PathBuf,
    extensions: &FileExtensions,
    _includes: &Option<Vec<String>>,
    _excludes: &Option<Vec<String>>,
) -> anyhow::Result<Vec<PathBuf>> {
    let dir = cwd.to_str().unwrap_or("");

    let glob_ext = |extension| -> anyhow::Result<_> {
        let pat = format!("{}/**/{}", dir, extension);
        let globs = glob::glob(&pat).map_err(|err| {
            anyhow::anyhow!(
                "{} Error at position: {} due to {}",
                emoji::ERROR,
                err.pos,
                err.msg
            )
        })?;

        Ok(globs.map(|glob_res| Ok(glob_res?)))
    };

    match extensions {
        FileExtensions::SingleFile(sfile) => glob_ext(sfile)?.collect(),
        FileExtensions::MultipleFile(strs) => {
            strs.iter()
                .map(glob_ext)
                .try_fold(Vec::new(), |mut v, globs| {
                    for glob in globs? {
                        v.push(glob?)
                    }
                    Ok(v)
                })
        }
    }
}

/// Convert each PathBuf into FileMeta
/// FileMeta consist of file's path and its modification times
pub fn path_to_filemeta(paths: Vec<PathBuf>) -> Result<BTreeSet<FileMeta>> {
    let mut filemeta = BTreeSet::new();
    for p in paths {
        let metadata = metadata(&p)?;
        let mtime = FileTime::from_last_modification_time(&metadata).unix_seconds();
        if let false = filemeta.insert(FileMeta {
            mtimes: mtime,
            path: p.clone(),
        }) {
            CLOG.warn(&format!("Duplicated file detected:"));
            CLOG.warn(&format!(" - {:?} ", p.display()));
            CLOG.warn(&format!(
                "Maybe you want to format one file with different formatter?"
            ));
            // return Err(anyhow!("prjfmt failed to run."));
        }
    }
    Ok(filemeta)
}

/// Creating command configuration based on prjfmt.toml
pub fn create_command_context(prjfmt_toml: &PathBuf) -> Result<Vec<CmdContext>> {
    let open_prjfmt = match read_to_string(prjfmt_toml.as_path()) {
        Ok(file) => file,
        Err(err) => {
            return Err(anyhow!(
                "cannot open {} due to {}.",
                prjfmt_toml.display(),
                err
            ))
        }
    };

    let cwd = match prjfmt_toml.parent() {
        Some(path) => path,
        None => {
            return Err(anyhow!(
                "{}prjfmt.toml not found, please run --init command",
                emoji::ERROR
            ))
        }
    };

    let toml_content: Root = toml::from_str(&open_prjfmt)?;
    let cmd_context: Vec<CmdContext> = toml_content
        .formatters
        .values()
        .map(|config| {
            let list_files = glob_to_path(
                &cwd.to_path_buf(),
                &config.files,
                &config.includes,
                &config.excludes,
            )?;
            Ok(CmdContext {
                command: config.command.clone().unwrap_or_default(),
                args: config.args.clone().unwrap_or_default(),
                metadata: path_to_filemeta(list_files)?,
            })
        })
        .collect::<Result<Vec<CmdContext>, Error>>()?;
    Ok(cmd_context)
}

/// prjfmt.toml structure
#[derive(Debug, Deserialize)]
pub struct Root {
    /// Map of formatters into the config
    pub formatters: BTreeMap<String, FmtConfig>,
}

/// Config for each formatters
#[derive(Debug, Deserialize)]
pub struct FmtConfig {
    /// File extensions that want to be formatted
    pub files: FileExtensions,
    /// File or Folder that is included to be formatted
    pub includes: Option<Vec<String>>,
    /// File or Folder that is excluded to be formatted
    pub excludes: Option<Vec<String>>,
    /// Command formatter to run
    pub command: Option<String>,
    /// Argument for formatter
    pub args: Option<Vec<String>>,
}

/// File extensions can be single string (e.g. "*.hs") or
/// list of string (e.g. [ "*.hs", "*.rs" ])
#[derive(Debug, Deserialize, Clone)]
#[serde(untagged)]
pub enum FileExtensions {
    /// Single file type
    SingleFile(String),
    /// List of file type
    MultipleFile(Vec<String>),
}

#[derive(Debug, Deserialize, Serialize, Clone)]
/// Each context of the formatter config
pub struct CmdContext {
    /// formatter command to run
    pub command: String,
    /// formatter arguments or flags
    pub args: Vec<String>,
    /// formatter target path
    pub metadata: BTreeSet<FileMeta>,
}

#[derive(Debug, Deserialize, Serialize, Clone)]
/// File metadata created after the first prjfmt run
pub struct FileMeta {
    /// Last modification time listed in the file's metadata
    pub mtimes: i64,
    /// Path to the formatted file
    pub path: PathBuf,
}

impl Ord for FileMeta {
    fn cmp(&self, other: &Self) -> Ordering {
        if let true = self.eq(other) {
            return Ordering::Equal;
        }
        if let true = self.mtimes.eq(&other.mtimes) {
            return self.path.cmp(&other.path);
        }
        self.mtimes.cmp(&other.mtimes)
    }
}

impl PartialOrd for FileMeta {
    fn partial_cmp(&self, other: &Self) -> Option<Ordering> {
        Some(self.cmp(other))
    }
}

impl PartialEq for FileMeta {
    fn eq(&self, other: &Self) -> bool {
        self.mtimes == other.mtimes && self.path == other.path
    }
}

impl Eq for FileMeta {}
