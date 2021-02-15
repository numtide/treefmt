#![allow(clippy::redundant_closure, clippy::redundant_pattern_matching)]

use treefmt::command::{run_cli, Cli};
use treefmt::CLOG;
use structopt::StructOpt;

fn main() {
    if let Err(e) = run() {
        CLOG.error(&format!("{}", e));
        ::std::process::exit(1);
    }
}

fn run() -> anyhow::Result<()> {
    let options = Cli::from_args();

    CLOG.set_log_level(options.log_level);

    if options.quiet {
        CLOG.set_quiet(true);
    }

    run_cli(options)?;

    Ok(())
}
