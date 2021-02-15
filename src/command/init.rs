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
        r#"# treefmt is the universal code formatter - https://github.com/numtide/treefmt
[formatter.<Language>]
includes = [ "*.<language-extension>" ]
excludes = []
command = ""
options = []
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
