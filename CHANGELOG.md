
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
