<p align=center>
  <img alt="Biscepter Logo" height=200 src="assets/logo.png"/>
</p>

<h2 align=center>Biscepter: Efficient Repeated and Concurrent Bisection</h2>

<p align=center>
  <img alt="License Badge" src="https://img.shields.io/github/license/DominicWuest/biscepter">
  <a href="https://pkg.go.dev/github.com/DominicWuest/biscepter?tab=doc"><img src="https://godoc.org/github.com/golang/gddo?status.svg" alt="Biscepter GoDoc"></a>
</p>

<p align=center>
  <a href="https://github.com/DominicWuest/biscepter/actions/workflows/build.yml"><img alt="CI/CD Build Status Badge" src="https://github.com/DominicWuest/biscepter/actions/workflows/build.yml/badge.svg"></a>
  <a href="https://github.com/DominicWuest/biscepter/actions/workflows/test.yml"><img alt="CI/CD Test Status Badge" src="https://github.com/DominicWuest/biscepter/actions/workflows/test.yml/badge.svg"></a>
  <a href="https://codecov.io/gh/DominicWuest/biscepter"><img alt="CI/CD Coverage Status Badge" src="https://codecov.io/gh/DominicWuest/biscepter/branch/main/graph/badge.svg?token=lY5KKsQlpx"/></a>
</p>

<details>

<summary><b>Table of Contents</b></summary>

- [🛠️ Why Biscepter over git bisect?](#️-why-biscepter-over-git-bisect)
- [⚙️ Installation](#-installation)
- [📡 API](#-api)
- [📦 Go Package](#-go-package)
- [🩺 Healthchecks](#-healthchecks)

</details>

---

# 🛠️ Why Biscepter over git bisect?

Biscepter and git bisect may solve the same problem, but biscepter comes with multiple advantages:
1. 🗄️ Caching of builds from previous bisects
1. 🚂 Bisecting multiple issues at once
1. 🚦 Possibility of prioritizing previously built commits to avoid arduous rebuilding
1. 🛡️ Automatically remembering and avoiding commits that break the build
1. 🩺 Easy builtin healthchecks to ensure the system under test is ready
1. 🐳 Simple definition of builds through Dockerfiles

Together, these advantages result in a more streamlined and straight-forward way of bisecting large systems exhibiting multiple different issues.

# ⚙️ Installation

Installation of the biscepter CLI requires Go `1.22.0` or newer.

</br>

Installing directly:
```
$ go install github.com/DominicWuest/biscepter@latest
```

You should now be able to run dinkel from the command line using `biscepter`.  
If you encounter an error, ensure that the `GOBIN` environment variable is set and in your path.

</br>

Alternatively, you may clone this repository and build biscepter locally
```
$ git clone git@github.com:DominicWuest/biscepter.git
$ cd biscepter
$ go build
```

You should now have a binary which you can run via `./biscepter`.

# 📡 API

The CLI exposes a RESTful HTTP API on port `40032` (can be changed via flags), whose openAPI specification can be found under [/api/openapi.yml](/api/openapi.yml).

For every kind of system to test, a job config has to be created.  
An example config with explanations of all fields can be found at [/configs/job-config.yml](/configs/job-config.yml).

Using this API, any language can be used to communicate with biscepter.
Be sure to check out the examples under [/examples/api-*](/examples) to get a quick understanding of how to use the API!

# 📦 Go Package

This repository contains a [Go package](/pkg/biscepter), whose documentation can be found [here](https://pkg.go.dev/github.com/DominicWuest/biscepter/pkg/biscepter).
Said package allows a Go program to control biscepter without a need for the CLI.  
Be sure to check out the examples under [/examples/pkg-*](/examples) to get a quick overview of how to use the Go package!

For every kind of system to test, a job config has to be created.  
An example config with explanations of all fields can be found at [/configs/job-config.yml](/configs/job-config.yml).  
It is easiest to create a job using such a config, but it can also be created by setting the job's fields manually.
The required fields for a job to run correctly are:
- `ReplicaCount`
- `GoodCommit` &amp; `BadCommit`
- `Dockerfile` or `DockerfilePath`
- `Repository`

# 🩺 Healthchecks

This section contains a list of all supported types of healthchecks, as well as what the additionally supplied data represents for this healthcheck.

| Type |  Explanation | Data | Data Example |
| --- | --- | --- | --- |
| HTTP | This endpoint is considered healthy if it returns a status code of `200` on a GET request. | The path of the URL | "/status" |
| Script| This endpoint is considered healthy if the script returns with exit code `0`. The environment variable `$PORT<XXXX>` can be used within the script to get the port to which `<XXXX>` was mapped to on the host (e.g. `$PORT443`). | The script to run | "echo Hello World!" |