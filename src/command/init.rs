use crate::config;
use anyhow::Context;
use console::style;
use log::info;
use std::fs;
use std::path::Path;

pub fn init_cmd(work_dir: &Path) -> anyhow::Result<()> {
    let file_path = work_dir.join(config::FILENAME);
    // TODO: detect if file exists
    fs::write(&file_path, std::include_bytes!("init_treefmt.toml")).with_context(|| {
        format!(
            "{} `{}`",
            style("Error writing").bold().red(),
            style(file_path.display()).bold()
        )
    })?;

    info!("Generated treefmt template at {}", file_path.display());
    Ok(())
}
