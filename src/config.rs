//! Contains the project configuration schema and parsing
use std::fs::read_to_string;
use std::path::PathBuf;
use std::{collections::BTreeMap, path::Path};

use anyhow::{Error, Result};
use serde::Deserialize;

/// Name of the config file
pub const FILENAME: &str = "treefmt.toml";
/// Alternative name of the config file
pub const FILENAMEALT: &str = ".treefmt.toml";

/// treefmt.toml structure
#[derive(Debug, Deserialize, Clone, Eq, PartialEq)]
pub struct Root {
    /// Config that applies to every formatter
    pub global: Option<GlobalConfig>,
    /// Map of formatters into the config
    pub formatter: BTreeMap<String, FmtConfig>,
}

/// Global config which applies to every formatter
#[derive(Debug, Deserialize, Clone, Eq, PartialEq)]
pub struct GlobalConfig {
    /// Global glob to exclude files or folder for all formatters
    #[serde(default)]
    pub excludes: Vec<String>,
}

/// Config for each formatters
#[derive(Debug, Deserialize, Clone, Eq, PartialEq)]
pub struct FmtConfig {
    /// Command formatter to run
    pub command: String,
    /// Working directory for formatter
    #[serde(default = "cwd")]
    pub work_dir: PathBuf,
    /// Argument for formatter
    #[serde(default)]
    pub options: Vec<String>,
    /// File or Folder that is included to be formatted
    #[serde(default)]
    pub includes: Vec<String>,
    /// File or Folder that is excluded to be formatted
    #[serde(default)]
    pub excludes: Vec<String>,
}

// The default work_dir value. It's a bit clunky. See https://github.com/serde-rs/serde/issues/1814
fn cwd() -> PathBuf {
    ".".into()
}

/// Returns an absolute path to the treefmt.toml file. From the current folder, and up.
#[must_use]
pub fn lookup(dir: &Path) -> Option<PathBuf> {
    let mut cwd = dir.to_path_buf();
    loop {
        let config_file = cwd.join(FILENAME);
        let config_file_alt = cwd.join(FILENAMEALT);
        if config_file.exists() {
            return Some(config_file);
        } else if config_file_alt.exists() {
            return Some(config_file_alt);
        }
        cwd = match cwd.parent() {
            Some(x) => x.to_path_buf(),
            // None is returned when .parent() is already the root folder. In that case we have
            // exhausted the search space.
            None => return None,
        };
    }
}

/// Loads the treefmt.toml config from the given file path.
pub fn from_path(file_path: &Path) -> Result<Root> {
    read_to_string(file_path)
        .map_err(Error::msg)
        .and_then(|content| from_string(&content))
}

/// Parses the provided string into a treefmt config object
pub fn from_string(file_contents: &str) -> Result<Root> {
    toml::from_str::<Root>(file_contents).map_err(Error::msg)
}

#[cfg(test)]
mod tests {
    use std::fs;
    use std::fs::File;

    use super::*;

    fn read_toml<T: for<'a> Deserialize<'a>>(toml: &str) -> Result<T, String> {
        toml::from_str::<T>(toml).map_err(|x| x.to_string())
    }

    fn read_root(toml: &str) -> Result<Root, String> {
        from_string(toml).map_err(|x| x.to_string())
    }

    fn default_root(command: &str) -> Root {
        Root {
            global: None,
            formatter: BTreeMap::from([(command.to_string(), default_fmt_config("sh"))]),
        }
    }

    fn default_global_config() -> GlobalConfig {
        GlobalConfig { excludes: vec![] }
    }

    fn default_fmt_config(command: &str) -> FmtConfig {
        FmtConfig {
            command: command.to_string(),
            work_dir: PathBuf::from("."),
            options: vec![],
            includes: vec![],
            excludes: vec![],
        }
    }

    #[test]
    fn test_cwd() {
        assert_eq!(true, cwd().is_relative(),);
        assert_eq!(Some("."), cwd().to_str());
    }

    #[test]
    fn test_global_config() {
        // default
        let toml = "";
        assert_eq!(Ok(default_global_config()), read_toml::<GlobalConfig>(toml));

        // parses correctly
        let toml = r#"
        excludes = ["foo", "bar", "baz"]
        "#;
        let excludes: Vec<String> = ["foo", "bar", "baz"].map(String::from).to_vec();
        let expected = GlobalConfig { excludes };
        assert_eq!(Ok(expected), read_toml::<GlobalConfig>(toml))
    }

    #[test]
    fn test_fmt_command() {
        // required
        assert_eq!(
            Err("missing field `command`".to_string()),
            read_toml::<FmtConfig>("")
        );
        // parses correctly
        assert_eq!(
            Ok(default_fmt_config("sh")),
            read_toml::<FmtConfig>(r#"command="sh""#)
        );
    }

    #[test]
    fn test_fmt_work_dir() {
        // defaults to cwd
        let toml = r#"
        command="sh"
        "#;
        let fmt_config = read_toml::<FmtConfig>(toml).unwrap();
        assert_eq!(PathBuf::from("."), fmt_config.work_dir);

        // parses correctly
        let spec = r#"
        command="sh"
        work_dir="/foo/bar"
        "#;
        let fmt_config = read_toml::<FmtConfig>(spec).unwrap();
        assert_eq!(PathBuf::from("/foo/bar"), fmt_config.work_dir);
    }

    #[test]
    fn test_fmt_options() {
        // defaults to empty
        let toml = r#"
        command="sh"
        "#;
        let fmt_config = read_toml::<FmtConfig>(toml).unwrap();
        let default_options: Vec<String> = Vec::new();
        assert_eq!(default_options, fmt_config.options);

        // parses correctly
        let spec = r#"
        command="sh"
        options=["foo", "bar", "baz"]
        "#;
        let fmt_config = read_toml::<FmtConfig>(spec).unwrap();
        assert_eq!(vec!["foo", "bar", "baz"], fmt_config.options);
    }

    #[test]
    fn test_fmt_includes() {
        // defaults to empty
        let toml = r#"
        command="sh"
        "#;
        let fmt_config = read_toml::<FmtConfig>(toml).unwrap();
        let default_includes: Vec<String> = Vec::new();
        assert_eq!(default_includes, fmt_config.options);

        // parses correctly
        let spec = r#"
        command="sh"
        includes=["foo", "bar", "baz"]
        "#;
        let fmt_config = read_toml::<FmtConfig>(spec).unwrap();
        assert_eq!(vec!["foo", "bar", "baz"], fmt_config.includes);
    }

    #[test]
    fn test_fmt_excludes() {
        // defaults to empty
        let toml = r#"
        command="sh"
        "#;
        let fmt_config = read_toml::<FmtConfig>(toml).unwrap();
        let default_excludes: Vec<String> = Vec::new();
        assert_eq!(default_excludes, fmt_config.options);

        // parses correctly
        let spec = r#"
        command="sh"
        excludes=["foo", "bar", "baz"]
        "#;
        let fmt_config = read_toml::<FmtConfig>(spec).unwrap();
        assert_eq!(vec!["foo", "bar", "baz"], fmt_config.excludes);
    }

    #[test]
    fn test_root() {
        // requires at least one formatter
        let toml = "";
        assert_eq!(
            Err("missing field `formatter`".to_string()),
            read_toml::<Root>(toml)
        );

        // global defaults to None
        let toml = r#"
        [formatter.sh]
        command = "sh"
        "#;

        let expected = default_root("sh");
        assert_eq!(Ok(expected), read_root(toml));

        // parses global correctly
        let toml = r#"
        [global]
        excludes = ["foo", "bar", "baz"]

        [formatter.sh]
        command = "sh"
        "#;
        let mut expected = default_root("sh");
        expected.global = Some(GlobalConfig {
            excludes: ["foo", "bar", "baz"].map(String::from).to_vec(),
        });

        assert_eq!(Ok(expected), read_root(toml));

        // formatters are keyed correctly in the formatters map
        let toml = r#"
        [formatter.sh]
        command = "sh"

        [formatter.rust]
        command = "rustfmt"

        [formatter.haskell]
        command = "cabal-fmt"
        "#;

        let mut expected = default_root("sh");
        expected.formatter = BTreeMap::from([
            ("sh".to_string(), default_fmt_config("sh")),
            ("rust".to_string(), default_fmt_config("rustfmt")),
            ("haskell".to_string(), default_fmt_config("cabal-fmt")),
        ]);

        assert_eq!(Ok(expected), read_root(toml));
    }

    #[test]
    fn test_from_path() {
        // sample toml
        let toml = r#"
        [global]
        excludes = ["foo", "bar", "baz"]

        [formatter.sh]
        command = "sh"
        "#;

        // create a temp toml file
        let root = tempfile::tempdir().unwrap();
        let file_path = root.as_ref().join(PathBuf::from("treefmt.toml"));

        fs::write(&file_path, toml).expect("failed to write toml to temp file");

        // utility for mapping into Result<Root, String> for easier assertions
        let read_from_path =
            |path: &Path| -> Result<Root, String> { from_path(path).map_err(|x| x.to_string()) };

        // bad path
        assert_eq!(
            Err("No such file or directory (os error 2)".to_string()),
            read_from_path(Path::new("foo/bar/baz.toml"))
        );

        // good path
        let mut expected = default_root("sh");
        expected.global = Some(GlobalConfig {
            excludes: ["foo", "bar", "baz"].map(String::from).to_vec(),
        });

        // the cwd directory during `cargo test` is the same directory as the crate under test
        // we do the following to ensure a consistent path regardless of running in IDE or terminal
        assert_eq!(Ok(expected), read_from_path(&file_path));
    }

    #[test]
    fn test_lookup() {
        // create temporary directory structure
        let root = tempfile::tempdir().unwrap();
        let level_one = tempfile::tempdir_in(&root).unwrap();
        let level_two = tempfile::tempdir_in(&level_one).unwrap();
        let level_three = tempfile::tempdir_in(&level_two).unwrap();
        let level_four = tempfile::tempdir_in(&level_three).unwrap();
        let level_five = tempfile::tempdir_in(&level_four).unwrap();

        // create temp config files using the standard and alternative filename at different
        // levels of the directory tree
        File::create(level_one.as_ref().join(PathBuf::from("treefmt.toml"))).unwrap();
        File::create(level_three.as_ref().join(PathBuf::from(".treefmt.toml"))).unwrap();

        let standard_path = level_one.as_ref().join(PathBuf::from("treefmt.toml"));
        let alternative_path = level_three.as_ref().join(PathBuf::from(".treefmt.toml"));

        // single and multi-level traversal to find alternative config file
        assert_eq!(Some(alternative_path.clone()), lookup(level_five.as_ref()));
        assert_eq!(Some(alternative_path.clone()), lookup(level_four.as_ref()));

        // single and multi-level traversal to find standard config file
        assert_eq!(Some(standard_path.clone()), lookup(level_two.as_ref()));
        assert_eq!(Some(standard_path.clone()), lookup(level_one.as_ref()));

        // fail to find
        assert_eq!(None, lookup(root.as_ref()));
    }
}
