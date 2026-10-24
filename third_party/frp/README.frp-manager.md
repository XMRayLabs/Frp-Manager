# frp-manager dependency patch

Source: https://github.com/fatedier/frp/tree/v0.71.1
Version: github.com/fatedier/frp v0.71.1 (Go module cache, verified by go.sum).
License: Apache-2.0; original LICENSE and source headers retained.

This directory contains the upstream assets, client, pkg and server directories,
including their tests, plus the original go.mod and go.sum. Standalone commands,
dashboards, documentation and end-to-end fixtures are not needed by this project.

Local change: pkg/util/net/conn.go StatsConn read/write byte counters use
atomic.Int64 Add/Load so concurrent HTTP reads, writes and connection closure
cannot race. The callback still runs once through the existing atomic close flag.
The protocol and configuration are unchanged.

Regression tests run from services/client/stats_conn_test.go, including under
`go test -race ./pkg/websiteauth ./services/client`. Remove the local replacement
when a verified upstream release includes this fix.
