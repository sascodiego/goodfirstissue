# httpie__cli

## What the project says it is (README)
HTTPie CLI: human-friendly HTTP client for the API era

HTTPie (pronounced _aitch-tee-tee-pie_) is a command-line HTTP client.
Its goal is to make CLI interaction with web services as human-friendly as possible.
HTTPie is designed for testing, debugging, and generally interacting with APIs & HTTP servers.
The `http` & `https` commands allow for creating and sending arbitrary HTTP requests.
They use simple and natural syntax and provide formatted and colorized output.

## We lost 54k GitHub stars

Please note we recently accidentally made this repo private for a moment, and GitHub deleted our community that took a decade to build. Read the full story here: https://httpie.io/blog/stardust

## Getting started

- Installation instructions →
- Full documentation →

## Features

- Expressive and intuitive syntax
- Formatted and colorized terminal output
- Built-in JSON support
- Forms and file uploads
- HTTPS, proxies, and authentication
- Arbitrary request data
- Custom headers
- Persistent sessions
- `wget`-like downloads

See all features →

## Examples

Hello World:

```bash
https httpie.io/hello
```

Custom HTTP method, HTTP headers and JSON data:

```bash
http PUT pie.dev/put X-API

## What it asks of contributions (CONTRIBUTING)
# Contributing to HTTPie

Bug reports and code and documentation patches are welcome. You can
help this project also by using the development version of HTTPie
and by reporting any bugs you might encounter.

## 1. Reporting bugs

**It's important that you provide the full command argument list
as well as the output of the failing command.**

Use the `--debug` flag and copy&paste both the command and its output
to your bug report, e.g.:

```bash
$ http --debug 

```

## 2. Contributing Code and D

## What it has been building lately (last 20 commit subjects)
5b604c3 Fix `https` behaviour in fish (#1611)
