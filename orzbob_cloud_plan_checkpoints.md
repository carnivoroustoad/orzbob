# Orzbob Cloud – Checkpoint‑Driven Implementation Plan

> **Objective**  
> Convert the existing local‑only **orzbob** tool into a production‑ready SaaS by completing a series of **bite‑sized, verifiable checkpoints**.  
> Each checkpoint is scoped to ≤ 1 day of focused work (ideally 2–4 h) and ends with an **automated or manual verification step** so a coding agent (human or AI) can prove completion before moving on.

---

## Legend

| Emoji | Meaning |
|-------|---------|
| 🏁 | Start here |
| ✅ | Verification / "Definition of Done" |
| ⏱️ | Expected effort |
| 🔗 | Dependency on previous checkpoint |

---

## Checkpoint Table

| # | Scope | Tasks | Verification (DONE = ✅) | Effort |
|---|-------|-------|--------------------------|--------|
| **C‑01** | **Repo Skeleton** | • Add `cmd/cloud-cp`, `cmd/cloud-agent`, `internal/cloud` packages.<br>• Update `go.mod` with `k8s.io/client-go`, `github.com/go-chi/chi/v5`. | ✅ `go vet ./...` & `go test ./...` still pass. | ⏱️ 2 h |
| **C‑02** 🔗C‑01 | **API Protobuf** | • Create `internal/cloud/api/instance.proto` per spec.<br>• Configure `buf.yaml`, add `make proto`. | ✅ `make proto` generates stubs; `go test ./...` compiles. | 2 h |
| **C‑03** 🔗C‑02 | **CLI Cloud Stub** | • Add `orz cloud new|attach|list|kill` with fake responses.<br>• Store OAuth token in `~/.config/orzbob/token.json`. | ✅ Running `orz cloud list` prints hard‑coded stub w/o panic. | 3 h |
| **C‑04** 🔗C‑01 | **Fake Provider (kind)** | • Implement `provider.LocalKind` creating pods in a local `kind` cluster.<br>• Add `make kind-up`. | ✅ `make e2e-kind` creates pod, status=Running, then deletes. | 4 h |
| **C‑05** 🔗C‑04 | **Runner Agent Skeleton** | • New binary runs `sleep 3600`.<br>• Dockerfile `docker/runner.Dockerfile` builds image. | ✅ `docker run runner:dev --help` exits 0. | 2 h |
| **C‑06** 🔗C‑05 | **PodSpec Builder** | • Generate Kubernetes pod with main + PVC.<br>• Unit‑test in `internal/scheduler/pod_test.go`. | ✅ `go test ./internal/scheduler/...` passes. | 3 h |
| **C‑07** 🔗C‑03 | **Control‑Plane CreateInstance** | • Wire REST→provider, return stub attach URL.<br>• Helm chart `charts/cp`. | ✅ `curl -XPOST :8080/v1/instances` returns JSON with id. | 4 h |
| **C‑08** 🔗C‑07 | **Attach WebSocket Tunnel** | • Implement `internal/tunnel/wsproxy.go` echoing stdin→stdout.<br>• CLI `orz attach` prints "connected". | ✅ Manual: type text, receives echo. | 3 h |
| **C‑09** 🔗C‑06 | **Bootstrap Repo Clone** | • Runner clones repo URL env var, checks out branch.<br>• Add integration test in kind. | ✅ `go run hack/smoke.go` sees repo files in pod. | 3 h |
| **C‑10** 🔗C‑09 | **tmux + Program Launch** | • Runner starts `tmux` session, launches `claude` placeholder (`bash -c 'echo hi; sleep infinity'`). | ✅ `orz attach` shows "hi". | 2 h |
| **C‑11** 🔗C‑10 | **Heartbeat & Idle Reaper** | • Runner POST heartbeat every 20 s.<br>• CP cron deletes pods idle > 30 min. | ✅ Unit test fakes time, ensures deletion called. | 3 h |
| **C‑12** 🔗C‑05 | **Init & onAttach Scripts** | • Parse `.orz/cloud.yaml`.<br>• Runner executes `setup.init` once, `onAttach` per attach. | ✅ Kind test asserts `marker_init_done` file exists. | 4 h |
| **C‑13** 🔗C‑12 | **Sidecar Services** | • Support `postgres` and `redis` sidecar containers.<br>• Health probes. | ✅ `psql` from main container connects to localhost. | 3 h |
| **C‑14** 🔗C‑07 | **K8s Secrets Storage** | • Endpoint `POST /v1/secrets` writes K8s Secret.<br>• Pod mounts via `envFrom:`. | ✅ Integration test reads env var inside pod. | 3 h |
| **C‑15** 🔗C‑08 | **Signed Attach URL (JWT)** | • CP signs short‑live ES256 JWT; tunnel validates. | ✅ Expired token (after 2 min) returns 401. | 2 h |
| **C‑16** 🔗C‑11 | **Quota Enforcement** | • Middleware counts active pods per org; deny > 2 (free). | ✅ Unit test: 3rd create returns 429. | 2 h |
| **C‑17** 🔗C‑14 | **Metrics & Logs** | • Expose `/metrics`; counters `active_sessions`.<br>• Fluent‑bit DaemonSet. | ✅ `curl /metrics` shows counter increasing. | 2 h |
| **C‑18** 🔗C‑17 | **CI/CD** | • Extend GitHub Actions: build images, helm‑lint, kind‑e2e.<br>• Push to GHCR. | ✅ `build.yml` passes on PR. | 4 h |
| **C‑19** 🔗C‑18 | **Docs & Quick‑Start** | • Update `README.md` with "Cloud Quick‑Start".<br>• Provide example `.orz/cloud.yaml`. | ✅ New dev follows doc and succeeds in < 15 min. | 2 h |
| **C‑20** 🔗All | **Beta Launch Checklist** | • Pen‑test scan, SOC 2 pre‑check.<br>• SLO dashboard.<br>• Free tier enable. | ✅ Internal dog‑fooding for one week w/ < 1 % error. | 1 d |

---

## Verification Scripts & Commands

* **Kind E2E:** `hack/e2e-kind.sh` creates cluster, deploys chart, runs smoke Go tests.  
* **Smoke Test Program:** `hack/smoke.go` ➜ create instance, attach, send "ping", expect "pong".  
* **Helm Dry‑Run:** `helm template charts/cp | kubectl apply --dry-run=client -f -`  
* **CI Status Badge:** `![build](https://github.com/…/actions/workflows/build.yml/badge.svg)`

---

## How to Use This File

1. Assign checkpoints to engineering resources (one owner each).  
2. After completing tasks, run the associated **verification**; record pass/fail in your tracker.  
3. Only move to the next checkpoint when the previous one shows ✅.  
4. Update this table in PRs to reflect real‑world time spent or new dependencies.

---

© 2025 Orzbob Inc. – Released under AGPL‑3.0 fileciteturn0file0