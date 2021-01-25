use crate::emoji;
use crate::CLOG;
use anyhow::Context;
use console::style;
use log::info;
use std::fs;
use std::path::PathBuf;

pub fn init_prjfmt(path: Option<PathBuf>) -> anyhow::Result<()> {
    info!("Creating new prjfmt configuration...");
    let file = match path {
        Some(loc) => loc,
        None => PathBuf::from("."),
    };
    let file_path = file.as_path().join("prjfmt.toml");
    // TODO: detect if file exists
    fs::write(
        file_path.as_path(),
        r#"# prjfmt is the universal code formatter - https://github.com/numtide/prjfmt
[formats.<Language>]
includes = [ "*.<language-extension>" ]
excludes = []
command = ""
args = []
    "#,
    )
    .with_context(|| {
        format!(
            "{} {} `{}`",
            emoji::ERROR,
            style("Error writing").bold().red(),
            style(file_path.display()).bold()
        )
    })?;

    let msg = format!("Generated prjfmt template at {}", file_path.display());
    CLOG.info(&msg);
    Ok(())
}
