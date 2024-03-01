# Overview

This example bisects three issues simultaneously using the Go pkg.
Each issue contains a merge commit, which biscepter finds, and then further bisects.

The code checks whether the endpoint `/i` returns `i`, with `i` being an integer between `3` and `5`, or something else.
A response of `i` indicates a good commit and all other responses indicate a bad one.

The bug on endpoint `3` was introduced by creating a merge commit using the `git` cli tool.  
For the bug on endpoint `4`, a pull request was created and accepted on GitHub's web interface.  
Lastly, the bug on endpoint `5` was introduced in a nested branch, meaning biscepter needs to bisect two merges to find the offending commit.

# Possible Output (Formatted for Clarity)

```
0: Got running system on port 44861 for replica with index 0
0: Got response -1
0: This commit is bad!
1: Got running system on port 33115 for replica with index 1
1: Got response 4
1: This commit is good!
2: Got running system on port 40395 for replica with index 2
2: Got response 5
2: This commit is good!

0: Got running system on port 45755 for replica with index 0
0: Got response 3
0: This commit is good!
1: Got running system on port 36217 for replica with index 1
1: Got response -1
1: This commit is bad!
2: Got running system on port 44819 for replica with index 2
2: Got response 5
2: This commit is good!

0: Got running system on port 41893 for replica with index 0
0: Got response 3
0: This commit is good!
1: Got running system on port 37839 for replica with index 1
1: Got response 4
1: This commit is good!
2: Got running system on port 43631 for replica with index 2
2: Got response 5
2: This commit is good!

1: Got running system on port 44221 for replica with index 1
1: Got response -1
1: This commit is bad!
2: Got running system on port 37309 for replica with index 2
2: Got response -1
2: This commit is bad!

2: Got running system on port 34883 for replica with index 2
2: Got response 5
2: This commit is good!

0: Bisection done for replica with index 0! Offending commit: db9cf6aa3a666e41e69f50a783e59d57af724877
Commit message: Add bug to /3
1: Bisection done for replica with index 1! Offending commit: 72cad4a376c41aa6f83720d195c34cda83d6e7db
Commit message: Add bug to /4
2: Bisection done for replica with index 2! Offending commit: cfad207f7deb9beb6855bc050d20d721945d30df
Commit message: Add bug to /5

Finished bisecting all issues!
```