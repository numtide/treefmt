use std::fmt;

/// List of Haskell formatters that are available
pub enum Hfmt {
    Brittany,
    Floskell,
    Fourmolu,
    Ormolu, // This will be the default Haskell formatter
    StylishHaskell,
}

impl fmt::Display for Hfmt {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        let s = match self {
            Hfmt::Brittany => "brittany",
            Hfmt::Floskell => "floskell",
            Hfmt::Fourmolu => "fourmolu",
            Hfmt::Ormolu => "ormolu",
            Hfmt::StylishHaskell => "stylish-haskell",
        };
        write!(f, "{}", s)
    }
}

/// Represents the set of formatter tools `allfmt` uses
pub enum Tools {
    /// Haskell formatter tool
    HaskellFmts(Hfmt),
    /// gofmt tools
    GoFmt,
    /// rustfmt tools
    RustFmt,
}

impl fmt::Display for Tools {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        let s = match self {
            Tools::HaskellFmts(hfmt) => format!("{}", hfmt),
            Tools::GoFmt => "gofmt".to_string(),
            Tools::RustFmt => "rustfmt".to_string(),
        };
        write!(f, "{}", s)
    }
}
