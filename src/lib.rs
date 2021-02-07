//! Your favorite all-in-one formatter tool!

#![deny(missing_docs)]
pub mod command;
pub mod customlog;
pub mod engine;
pub mod emoji;
pub mod formatters;

use customlog::CustomLogOutput;

/// The global custom log and user-facing message output.
pub static CLOG: CustomLogOutput = CustomLogOutput::new();
