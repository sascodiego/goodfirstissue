# urfave/cli

## What the project says it is (README)
# Welcome to urfave/cli

[![Go Reference][goreference_badge]][goreference_link]
[![Go Report Card][goreportcard_badge]][goreportcard_link]
[![codecov][codecov_badge]][codecov_link]
[![Tests status][test_badge]][test_link]

urfave/cli is a **declarative**, simple, fast, and fun package for building
command line tools in Go featuring:

- commands and subcommands with alias and prefix match support
- flexible and permissive help system
- dynamic shell completion for `bash`, `zsh`, `fish`, and `powershell`
- no dependencies except Go standard library
- input flags for simple types, slices of simple types, time, duration, and
  others
- compound short flag support (`-a` `-b` `-c` can be shortened to `-abc`)
- documentation generation in `man` and Markdown (supported via the
  [`urfave/cli-docs`][urfave/cli-docs] module)
- input lookup from:
  - environment variables
  - plain text files
  - structured file formats (supported via the
    [`urfave/cli-altsrc`][urfave/cli-altsrc] module)

## Documentation

See the hosted documentation website at <https://cli.urfave.org>. Contents of
this website are built from the [`./docs`](./docs) directory.

## Build tags

You can use the following buil

## What it has been building lately (last 20 commit subjects)
Merge pull request #2536 from urfave/dependabot/pip/python-packages-1e57fd124c
build(deps): bump mkdocs-git-revision-date-localized-plugin
Merge pull request #2514 from nickajacks1/slow-completion
test: check bash completion does not expand candidates
perf: faster bash completion with many matches
Merge pull request #2512 from nk-batman21/fix/completion-help-command-name
Merge pull request #2506 from vidigoat/fix/fish-persistent-flags
Merge pull request #2511 from Rohilalala/fix/wrap-runes
fix: count runes, not bytes, when wrapping help text
fix: omit ancestor flags when a fish command skips flag parsing
Merge pull request #2501 from vidigoat/fix/complete-commands-after-finished-flag
Complete subcommands after a complete flag on the root command
Merge pull request #2502 from vidigoat/fix/complete-no-flag-after-its-alias
Merge pull request #2523 from urfave/dependabot/pip/python-packages-3f52cf6f4a
build(deps): bump the python-packages group with 2 updates
Merge pull request #2520 from 10kartik/help-nested-path
fix: show help for the whole subcommand path given to help
Suggest no flags after a short flag written out in full
Merge pull request #2489 from urfave/root-completion-run-args
fix: use the configured completion command name in its help
