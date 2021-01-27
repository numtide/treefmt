#![allow(clippy::redundant_closure, clippy::redundant_pattern_matching)]

extern crate anyhow;
extern crate env_logger;
extern crate log;
extern crate prjfmt;
extern crate structopt;

use prjfmt::{run_cli, Cli, CLOG};
use structopt::StructOpt;

fn main() {
    env_logger::init();
    if let Err(e) = run() {
        eprintln!("Error: {}", e);
        ::std::process::exit(1);
    }
}

fn run() -> anyhow::Result<()> {
    let args = Cli::from_args();

    CLOG.set_log_level(args.log_level);

    if args.quiet {
        CLOG.set_quiet(true);
    }

    run_cli(args)?;

    Ok(())
}
