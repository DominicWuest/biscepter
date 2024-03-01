## Overview

This example bisects three issues simultaneously using the Go pkg.

The code checks whether the endpoint `/i` returns `i`, with `i` being an integer between `0` and `2`, or something else.
A response of `i` indicates a good commit while any other response indicates a bad one.

This example also showcases biscepter's caching behavior, as well as how biscepter may choose to test nearby, already built, commits if it would save time over building the commit in the middle of the good and bad commit.

When running the example for the first time, replica 1 requires three iterations each to bisect the issue.
After then removing the image of the middle commit, the one that would be tested first by all replicas, replica 1 now only takes two iterations to find the offending commit.
This happens because biscepter chooses to not rebuild the middle image, but rather use an already built image because of the high build cost specified in `jobConf.yml`.
Reducing the build cost to 0 results in biscepter rebuilding the commit and replica 1 taking three iterations again.

## Possible Output (Formatted for Clarity)

Running for the first time:
```
0: Got running system on port 40117 for replica with index 0
0: Got response -1
0: This commit is bad!
1: Got running system on port 33545 for replica with index 1
1: Got response -1
1: This commit is bad!
2: Got running system on port 35361 for replica with index 2
2: Got response -1
2: This commit is bad!

0: Got running system on port 45957 for replica with index 0
0: Got response -1
0: This commit is bad!
1: Got running system on port 33589 for replica with index 1
1: Got response 1
1: This commit is good!
2: Got running system on port 44185 for replica with index 2
2: Got response 2
2: This commit is good!

1: Got running system on port 41475 for replica with index 1
1: Got response -1
1: This commit is bad!
2: Got running system on port 37169 for replica with index 2
2: Got response 2
2: This commit is good!

0: Bisection done for replica with index 0! Offending commit: 03cdf844a180c44763e12f29901ab5f8d61444f3
Commit message: Add bug to /0
1: Bisection done for replica with index 1! Offending commit: 22a405d30a6c8d3eb045062ac2be4cff57e30d29
Commit message: Add bug to /1
2: Bisection done for replica with index 2! Offending commit: 9b70eda4f3e48d5d906f99b570a16d5a979b0a99
Commit message: Add bug to /2

Finished bisecting all issues!
```

---

Running for the second time after removing the image of the middle commit using `docker rmi biscepter-9b70eda4f3e48d5d906f99b570a16d5a979b0a99`:
```
0: Got running system on port 45699 for replica with index 0
0: Got response -1
0: This commit is bad!
1: Got running system on port 35765 for replica with index 1
1: Got response -1
1: This commit is bad!
2: Got running system on port 38473 for replica with index 2
2: Got response 2
2: This commit is good!

0: Got running system on port 39359 for replica with index 0
0: Got response -1
0: This commit is bad!
1: Got running system on port 46071 for replica with index 1
1: Got response 1
1: This commit is good!
2: Got running system on port 34117 for replica with index 2
2: Got response -1
2: This commit is bad!

2: Got running system on port 44717 for replica with index 2
2: Got response -1
2: This commit is bad!

0: Bisection done for replica with index 0! Offending commit: 03cdf844a180c44763e12f29901ab5f8d61444f3
Commit message: Add bug to /0
1: Bisection done for replica with index 1! Offending commit: 22a405d30a6c8d3eb045062ac2be4cff57e30d29
Commit message: Add bug to /1
2: Bisection done for replica with index 2! Offending commit: 9b70eda4f3e48d5d906f99b570a16d5a979b0a99
Commit message: Add bug to /2

Finished bisecting all issues!
```

---

Running for the second time after removing the image of the middle commit using `docker rmi biscepter-9b70eda4f3e48d5d906f99b570a16d5a979b0a99`, but setting buildCost in `jobConf.yml` to `0`:
```
0: Got running system on port 46163 for replica with index 0
0: Got response -1
0: This commit is bad!
1: Got running system on port 34669 for replica with index 1
1: Got response -1
1: This commit is bad!
2: Got running system on port 42279 for replica with index 2
2: Got response -1
2: This commit is bad!

0: Got running system on port 45261 for replica with index 0
0: Got response -1
0: This commit is bad!
1: Got running system on port 44657 for replica with index 1
1: Got response 1
1: This commit is good!
2: Got running system on port 37883 for replica with index 2
2: Got response 2
2: This commit is good!

1: Got running system on port 34109 for replica with index 1
1: Got response -1
1: This commit is bad!
2: Got running system on port 45941 for replica with index 2
2: Got response 2
2: This commit is good!

0: Bisection done for replica with index 0! Offending commit: 03cdf844a180c44763e12f29901ab5f8d61444f3
Commit message: Add bug to /0
1: Bisection done for replica with index 1! Offending commit: 22a405d30a6c8d3eb045062ac2be4cff57e30d29
Commit message: Add bug to /1
2: Bisection done for replica with index 2! Offending commit: 9b70eda4f3e48d5d906f99b570a16d5a979b0a99
Commit message: Add bug to /2

Finished bisecting all issues!
```