#!/usr/bin/env bash
# Rewrites competitor benchmark names from Benchmark<Router><Category>-N to
# Benchmark<Category>/router=<Router>-N. Measured values are not changed.
# Longer router prefixes are matched first (MuxMasterPooled before MuxMaster).
exec sed -E 's/^Benchmark(MuxMasterPooled|MuxMasterFast|MuxMaster|HTTProuter|BunRouter|Chi|GorillaMux)([A-Za-z0-9]+)(-[0-9]+)?([[:space:]])/Benchmark\2\/router=\1\3\4/'
