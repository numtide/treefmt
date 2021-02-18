use crate::CLOG;
use anyhow::Context;
use console::style;
use std::fs;
use std::path::PathBuf;

pub fn init_cmd(path: Option<PathBuf>) -> anyhow::Result<()> {
    let file = match path {
        Some(loc) => loc,
        None => PathBuf::from("."),
    };
    let file_path = file.join("treefmt.toml");
    // TODO: detect if file exists
    fs::write(
        &file_path,
        r#"# One CLI to format the code tree - https://github.com/numtide/treefmt

[formatter.mylanguage]
# Formatter to run
command = "command-to-run"
# Command-line arguments for the command
options = []
# Glob pattern of files to include
includes = [ "*.<language-extension>" ]
# Glob patterns of files to exclude
excludes = []
    "#,
    )
    .with_context(|| {
        format!(
            "{} `{}`",
            style("Error writing").bold().red(),
            style(file_path.display()).bold()
        )
    })?;

    CLOG.info(&format!(
        "Generated treefmt template at {}",
        file_path.display()
    ));
    Ok(())
}
