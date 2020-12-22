use crate::emoji;
use crate::CLOG;
use anyhow::Context;
use console::style;
use log::info;
use std::fs;
use std::path::Path;

pub fn init_fmt() -> anyhow::Result<()> {
    info!("Creating new allfmt configuration...");

    let file = &format!("fmt.toml");
    let file_path = Path::new(file);
    // TODO: detect if file exists
    fs::write(
        file_path,
        r#"# fmt is the universal code formatter - https://github.com/numtide/fmt
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

    let msg = format!("Generated allfmt template at ./.");
    CLOG.info(&msg);
    Ok(())
}
