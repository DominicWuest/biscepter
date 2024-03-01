## Overview

This directory contains some examples which make use of biscepter.

Every example contains another README, which goes into more detail as to how the example operates as well as what can be observed when running the example.

Subdirectories starting in `pkg` use the Go-library directly for interacting with biscepter, whereas subdirectories starting in `api` use the HTTP API which the biscepter cli exposes.

## Examples

- `pkg-single`: Uses the Go package for bisecting a single issue.
- `pkg-multiple`: Uses the Go package for bisecting three issues at once.
- `pkg-multiple-merge`: Uses the Go package for bisecting three issues at once, each requiring further bisection of merge commits.
---
- `api-single`: Uses the HTTP API for bisecting a single issue in Python.
- `api-multiple`: Uses the HTTP API for bisecting three issues at once in Python.