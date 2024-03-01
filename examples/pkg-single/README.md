## Overview

This example bisects a single issue using the Go pkg.

The code checks whether the endpoint `/1` returns `1` or something else.
A response of `1` indicates a good commit while any other response indicates a bad one.

## Possible Output (Formatted for Clarity)

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