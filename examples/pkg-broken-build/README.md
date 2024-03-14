## Overview

This example technically doesn't bisect an issue, but rather shows how biscepter avoids commits whose builds are broken.

The commits bisected have the following structure:
```
2: Fixes Build
1: Breaks Build
0: Normal Commit
```

Since `1` is the only commit that biscepter needs to check, it attempts to build and test it.  
However, the build fails, and biscepter now blocks this commit from being built again, resorting to building commit `2`.

This is also concurrency save, meaning a replica attempting to build this broken commit simultaneously to another replica will result in both resorting to using the same replacement commit instead.

Notice that after the first run, a file, `.biscepter-replacements~` is created in your PWD.  
This file stores the replacements for broken commits, such as `2` in this case.
When now rerunning the demo, biscepter will already ignore the broken commit and immediately skip to building the replacement instead.

## Possible Output (Formatted for Clarity)

```
0: Image build of biscepter-bff09704de05273d4ff533a0acbdaed1218eb07d:d69cd7812bbea0cf96e52517828c13704e1d6e38f60baf96c0af791b0993c9b4 for commit hash bff09704de05273d4ff533a0acbdaed1218eb07d failed, avoiding commit from now on. Build output:
0: {"stream":"Step 1/5 : FROM golang:1.22.0-alpine"}
0: {"stream":"Step 2/5 : WORKDIR /app"}
0: {"stream":"Step 3/5 : COPY . ."}
0: {"stream":"Step 4/5 : RUN go build -o server main.go"}
0: {"errorDetail":{"code":1,"message":"The command '/bin/sh -c go build -o server main.go' returned a non-zero code: 1"},"error":"The command '/bin/sh -c go build -o server main.go' returned a non-zero code: 1"}

1: Image for commit hash bff09704de05273d4ff533a0acbdaed1218eb07d reported to be broken, reattempting to init next system.

0: Bisection done!
1: Bisection done!

1: Bisection avoided broken commit and reports the bad commit as being "e9bdabf4c4eb087645705fbd8c26f52bfab6aec8".
1: Commit message: "Fix build"
```