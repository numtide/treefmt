# Contribution Guidelines for the treefmt Project

Thank you for considering contributing to `treefmt`. This file contains instructions that will help you to make a contribution.

## Before Contributing

Before sending your pull requests, make sure that you **read the whole guidelines**. If you have any doubt on the contributing guide, please feel free to [state it clearly in an issue](https://github.com/numtide/treefmt/issues/new) or ask the community in [treefmt matrix server](https://matrix.to/#/#treefmt:numtide.com).

Use your best judgement, and feel free to propose changes to this document in a pull request.

## Guidelines for Developers

### First contribution?

Please take a few minutes to read GitHub's guide on [How to Contribute to Open Source](https://opensource.guide/how-to-contribute/).
It's a quick read, and it's a great way to introduce yourself to how things work behind the scenes in open-source projects.

### Getting started

- Make sure you have a [GitHub account](https://github.com/join).
- Take a look at [existing issues](https://github.com/numtide/treefmt/issues).
- If you need to create an issue:
  - Make sure to clearly describe it.
  - Including steps to reproduce when it is a bug.
  - Include the version of `treefmt` used.
  - Include the database driver and version.
  - Include the database version.

### Making changes

- Fork the repository on GitHub.
- Create a branch on your fork.
  - You can usually base it on the `master` branch.
  - Make sure not to commit directly to `master`.
- Make commits of logical and atomic units.
- Make sure you have added the necessary tests for your changes.
- Push your changes to a topic branch in your fork of the repository.
- Submit a pull request to the original repository.

#### Examples of git history

<details>
<summary>Git history that we want to have</summary>

```
*   e3ed88b (HEAD -> contribution-guide, upstream/master, origin/master, origin/HEAD, master) Merge pull request #470 from zimbatm/fix_lru_cache
|\
| * 1ab7d9f Use rayon for multithreading command
|/
*   e9c5bb4 Merge pull request #468 from zimbatm/multithread
|\
| * de2d6cf Add lint property for Formatter struct
| * cd2ed17 Fix impl on Formatter get_command() function
|/
*   028c344 Merge pull request #465 from rayon/0.15.0-release
|\
| * 7b619d6 0.15.0 release
|/
*   acdf7df Merge pull request #463 from zimbatm/support-multi-part-namespaces
```

</details>

<details>
<summary>Git history that we are <b>trying</b> to avoid</summary>

```
*   4c8aca8 Merge pull request #120 from zimbatm/add-rayon
|\
| * fc2b449 use rayon for engine now
| * 2304683 add rayon config
| * 5285bd3 bump base image to F30
* |   4d0fbe2 Merge pull request #114 from rizary/create_method_create_release
|\ \
| * | 36a9396 test changed
| * | 22f681d method create release for github created
* | |   2ef4ea1 Merge pull request #119 from rizary/config.rs
|\ \ \
| |/ /
|/| |
| * | 5f1b8f0 unused functions removed
* | |   a93c361 Merge pull request #117 from zimbatm/add-getreleases-to-abstract
|\ \ \
| |/ /
|/| |
| * | 0a97236 add get_releses for Cargo
| * | 55e4c57 add get_releases/get_release into engine.rs
|/ /
* |   badeddd Merge pull request #101 from zimbatm/extreme-cachin
```

</details>

### Checkers/linters/formatters

Run `treefmt` in the source directory before you commit your changes.

---

Additionally, it's always good to work on improving/adding examples and documentation.

Thank you for your interest!
