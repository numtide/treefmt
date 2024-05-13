---
outline: deep
---

# Contribution guidelines

This file contains instructions that will help you make a contribution.

## Licensing

The `treefmt` binaries and this user guide are licensed under the [MIT license](https://github.com/numtide/treefmt/blob/main/LICENSE).

## Before you contribute

Here you can take a look at the [existing issues](https://github.com/numtide/treefmt/issues). Feel free to contribute, but make sure you have a
[GitHub account](https://github.com/join) first :slightly_smiling_face:.

If you're new to open source, please read GitHub's guide on [How to Contribute to Open Source](https://opensource.guide/how-to-contribute/). It's a quick read,
and it's a great way to introduce yourself to how things work behind the scenes in open-source projects.

Before sending a pull request, make sure that you've read all the guidelines. If you don't understand something, please
[state your question clearly in an issue](https://github.com/numtide/treefmt/issues/new) or ask the community on the [treefmt matrix server](https://matrix.to/#/#treefmt:numtide.com).

## Creating an issue

If you need to create an issue, make sure to clearly describe it, including:

-   The steps to reproduce it (if it's a bug)
-   The version of `treefmt` used

The cache database is stored in a `.db` file in the `~/.cache/treefmt/eval-cache` directory.

## Making changes

If you want to introduce changes to the project, please follow these steps:

-   Fork the repository on GitHub
-   Create a branch on your fork. Don't commit directly to main
-   Add the necessary tests for your changes
-   Run `treefmt` in the source directory before you commit your changes
-   Push your changes to the branch in your repository fork
-   Submit a pull request to the original repository

Make sure you based your commits on logical and atomic units!

## Examples of git history

<details>

<summary>Git history that we want to have</summary>

```

*   e3ed88b (HEAD -> contribution-guide, upstream/main, origin/main, origin/HEAD, main) Merge pull request #470 from zimbatm/fix_lru_cache

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

Additionally, it's always good to work on improving documentation and adding examples.

Thank you for considering contributing to `treefmt`.
