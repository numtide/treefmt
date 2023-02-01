# About the project

`Treefmt` is a formatting tool that saves you time: it provides developers with a universal way to trigger all formatters needed for the project in one place.

## Background

Typically, each project has its own code standards enforced by the project's owner. Any code contributions must match that given standard, i.e. be formatted in a specific manner.

At first glance, the task of code formatting may seem trivial: the formatter can be automatically triggered when you save a file in your IDE. Indeed, formatting doesn't take much effort if you're working on a single project long term: setting up the formatters in your IDE won't take much of your time, and then you're ready to go.

Contrary to that, if you're working on multiple projects at the same time, you may have to update your formatter configs in the IDE each time you switch between the projects. This is because formatter settings aren't project-specific --- they are set up globally for all projects.

Alternatively, you can trigger formatters manually, one-by-one or in a script. Actually, for bigger projects, it's common to have a script that runs over your project's directories and calls formatters consequently. But it takes time to iterate through all the files.

All the solutions take up a significant amount of time which a developer could spend doing the actual work. They also require you to remember which formatters and options are used by each project you are working on.

`Treefmt` solves these issues.

## Why treefmt?

`Treefmt`'s configuration is project-specific, so you don't need to re-configure formatters each time you switch between projects, like you have to when working with formatters in the IDE.

Contrary to calling formatters from the command line, there's no need to remember all the specific formatters required for each project. Once you set up the config, you can run the tool in any of your project's folders without any additional flags or options.

Typically, formatters have different ways to say there was a specific error. With `treefmt`, you get a standardized output which is easier to understand than the variegated outputs of different formatters, so it takes less time to grasp what's wrong.

In addition, `treefmt` works faster than the custom script solution because the changed files are cached and the formatters run only against them. Moreover, formatters are run in parallel, which makes the tool even faster.

The difference may not be significant for smaller projects, but it gets quite visible as the project grows. For instance, take the caching optimization. It takes 9 seconds to traverse a project of 1507 files and no changes without caching:

```
traversed 1507 files
matched 828 files to formatters
left with 828 files after cache
of whom 0 files were re-formatted
all of this in 9s
```

...while it takes 124 milliseconds to traverse the same project with caching:

```
traversed 1507 files
matched 828 files to formatters
left with 0 files after cache
of whom 0 files were re-formatted
all of this in 124ms
```

The tool can be invoked manually or integrated into your CI. There's currently no integration with IDEs, but the feature is coming soon.

## Upcoming features

- **IDE integration:** Most of developers are used to formatting a file upon save in the IDE. So far, you can't use `treefmt` for this purpose, but we're working on it 😀
- **Pre-commit hook:** It's good to have your code checked for adherence to the project's standards before commit. `Treefmt` pre-commit hook won't let you commit if you have formatting issues.
- **Support of multiple formatters for one language:** In the current version, we advise you to avoid using multiple formatters for one and the same file type. This is because formatters are run in parallel and therefore may encounter issues while processing files. We are going to fix this issue soon, since there are cases when you may need more than one formatter per language.

As a next step, learn how to [install] and [use] `treefmt`.

[install]: installation.md
[use]: usage.md
