TODO: Add description here of example

Run once, notice all 3 replicas take 3 iterations.
Then remove middle image, `docker rmi biscepter-45b648f6f13a03f1c17847a83abbae4e39f554cf`.
Notice that 2 now only takes 2 iterations because of the cache and biscepter then determining that the lower commit induces lower cost!
