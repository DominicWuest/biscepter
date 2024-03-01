# Overview

This example bisects a single issue using the HTTP API, interacted with using Python and the OpenAPI spec in [`/api/openapi.yml`](../../api/openapi.yml).

The code checks whether the endpoint `/1` returns `1` or something else.
A response of `1` indicates a good commit while any other response indicates a bad one.

Requirements:
- [poetry](https://python-poetry.org)
- [openapi-python-client](https://github.com/openapi-generators/openapi-python-client)

To run this example, issue the following commands:
```bash
openapi-python-client generate --path ../../api/openapi.yml # Generate the openAPI python client
poetry install # Install client
# In a separate console, run `biscepter bisect` with the jobConf.yml in this directory
poetry run python main.py # Run the API client
```

# Possible Output (Formatted for Clarity)

```
Got running system on port 37333
Got response 1
This commit is good!

Got running system on port 38871
Got response -1
This commit is bad!

Bisection done! Offending commit: 22a405d30a6c8d3eb045062ac2be4cff57e30d29
Commit message: Add bug to /1
```