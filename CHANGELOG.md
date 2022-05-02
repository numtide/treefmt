
0.4.0 / 2022-05-02
==================

  * Cargo update (#158)
  * cli: add --no-cache option (#157)
  * cache: also track file sizes (#155)
  * build(deps): bump actions/download-artifact from 2 to 3 (#152)
  * build(deps): bump actions/upload-artifact from 2 to 3 (#151)
  * build(deps): bump cachix/install-nix-action from 16 to 17 (#150)
  * build(deps): bump actions/cache from 2 to 3 (#148)
  * README: link to wiki (#147)
  * build(deps): bump actions/checkout from 2 to 3 (#146)
  * website: remove landing page (#145)
  * Update rust.yml
  * nix: add mdsh to the devenv
  * treefmt.toml: fix shell invocation

0.3.0 / 2021-12-17
==================

  * formatter: noop on zero args
  * treefmt.toml: add shellcheck example
  * doc: add inline script example
  * Handle the case where no formatter match path for stdin (#138)
  * Trade in naersk for a newer version of nixpkgs (#134)
  * Add meta.description to play nicely with devshell menu (#132)
  * allow config files to be independent from worktree location (#131)
  * display round ms

0.2.6 / 2021-08-31
==================

  * display formatter outputs on error

0.2.5 / 2021-08-14
==================

  * process 1024 files at a time

0.2.4 / 2021-08-14
==================

  * collect the mtime *after* formatting.

0.2.3 / 2021-08-04
==================

  * engine: skip over symlinks (#123)
  * Support global excludes (#121)

0.2.2 / 2021-05-31
==================

  * print the executed formatters in debug mode
  * customlog: remove emojis

0.2.1 / 2021-05-08
==================

  * change default loglevel to INFO (#109)

0.2.0 / 2021-05-07
==================

  * support relative commands
  * document terraform fmt workaround
  * Always expand the path given in treefmt.toml (#107)
  * Update formatters-spec.md

0.1.1 / 2021-04-24
==================

  * Report formatter output on error (#104)

0.1.0 / 2021-04-10
==================

First release!
