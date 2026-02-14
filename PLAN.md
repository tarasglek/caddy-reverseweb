# CI Failure Fix Plan

## Status legend
- [ ] pending
- [x] done
- [~] in progress / attempted but blocked

## Current failures

### 1) `Lint Go` workflow fails at `govulncheck`
`govulncheck` is correctly running and failing the build because reachable vulns are reported through Caddy dependency paths.

Key items reported in logs include:
- `golang.org/x/crypto` (GO-2025-4116)
- `github.com/quic-go/quic-go` (GO-2025-4017, GO-2025-3735)
- `github.com/go-jose/go-jose/v3` (GO-2025-3485)
- `github.com/golang/glog` (GO-2025-3372)

### 2) `Test Go` workflow fails in integration tests
`TestDynamicDiscovery` fails in CI because required runtime binary is missing:
- `exec: "landrun": executable file not found in $PATH`

---

## Execution checklist

### A) Dependency upgrades
- [x] Upgrade dependencies to latest feasible versions (`go get -u ./...` + tidy).
- [~] Keep dependency graph build-stable after upgrades.
  - Current blocker: local tests now fail in `TestProcessCrashAndRestart` with a transient `502` after backend crash/restart.

### B) Local verification before push
- [x] Re-run local tests in tmux after upgrade attempt.
- [x] Achieve green local `go test ./...`.
  - Fixed race by detecting dead backend process before proxying and forcing restart.

### C) CI fixes
- [x] Ensure CI installs all runtime dependencies required by integration tests.
  - Added workflow install/verify for `uv`, `jq`, and `landrun` before `go test ./...`.
- [x] Keep tests strict (no skip-on-missing-tool fallback) once installs are in place.
- [~] Re-run CI and confirm `Test Go` passes.
  - Local fix applied: made `TestDynamicDiscovery` use a self-contained detector script (no `uv` network download dependency), and local `go test ./...` now passes.

### D) Vulnerability cleanup
- [x] Re-run `govulncheck ./...` after dependency stabilization.
- [~] Resolve/mitigate remaining reachable vulnerabilities.
  - Mitigated `github.com/slackhq/nebula` by upgrading to `v1.9.7`.
  - Remaining reachable set is currently stdlib `crypto/x509` advisories (fixed in Go >=1.25.5).
- [ ] Re-run CI and confirm `Lint Go` passes.

### E) Refactor: unify backend startup/restart path (remove duplicated startup logic)
- [ ] First priority: refactor runtime process/restart path so dependency bump to `github.com/smallstep/certificates@v0.29.0` remains stable (eliminate transient post-crash 502 race).
- [ ] Introduce a single helper for "ensure process running + ready + upstream resolved".
- [ ] Route both initial startup and restart-on-dead-process through this helper.
- [ ] Keep lock boundaries explicit (`ps.mu`) and avoid side effects in multiple call sites.
- [ ] Add/adjust tests for:
  - first request startup
  - crash/restart path
  - readiness timeout/failure path
- [ ] Verify no behavior regressions (`go test ./...` local + CI).

---

## Next immediate step
1. Re-check CI for commit `b4fb10c` and capture remaining failures.
2. Continue dependency/vuln cleanup based on current CI output.
3. Execute startup-path unification refactor (Section E) after CI is stable.
