# Orzbob Cloud – Billing Integration Roadmap (Polar.sh)

This checklist breaks the work into **nine concrete checkpoints** to introduce the new “$20 base + usage” pricing model (plus an optional pure usage plan) using [Polar.sh](https://www.polar.sh).  
Each checkpoint contains:

* **Goal** – what must be accomplished.  
* **Done when** – objective verification steps to prove completion.

---

| # | Checkpoint | Goal | Done when – verification checklist |
|---|------------|------|------------------------------------|
| 1 | **Polar project & products created** | • Create a Polar.sh project for *Orzbob Cloud*.<br>• Add 3 products (SKUs):<br> 1. `free-tier` – $0, 10 h/mo included.<br> 2. `base-plus-usage` – $20/mo, 200 **small‑tier** hours included.<br> 3. `usage‑only` – hidden, $0/mo, pay‑go rates in the meter. | ☐ Polar dashboard shows the three SKUs with correct display names & pricing.<br>☐ `usage‑only` is flagged **private / hidden**.<br>☐ Test checkout in Polar sandbox succeeds for each SKU. |
| 2 | **Secrets & env‑vars wired** | • Store **POLAR_API_KEY** in Orzbob secrets (Kubernetes `Secret` & local `.env`).<br>• Add **POLAR_WEBHOOK_SECRET** for signature verification. | ☐ `kubectl get secret polar-credentials -n orzbob-system` returns key names.<br>☐ `go test ./internal/billing -run TestPolarClientAuth` passes. |
| 3 | **Metering service skeleton** | • New Go package `internal/billing` with Polar SDK wrapper.<br>• Accept usage samples: `orgID, minutes, tier`.<br>• Batch & flush to Polar every 60 s. | ☐ `go test ./internal/billing -run TestBatchFlush` passes.<br>☐ Prometheus gauge `orzbob_usage_meter_queue` stays below 1 k after 10 min soak. |
| 4 | **Control‑plane hooks emit usage** | • Emit `minutes_used` every time instance status toggles **Running→Stopped** or heartbeat times out.<br>• Tier → price mapping: small = 8.3 ¢/h, medium = 16.7 ¢/h, gpu = $2.08/h. | ☐ Unit test `TestUsageEmissionOnStop` added & green.<br>☐ Local run shows POST to `/polar/meters` in control‑plane logs. |
| 5 | **Quota engine (included hours)** | • Persist monthly usage per org.<br>• Deduct from included 200 h (or 10 h).<br>• When exhausted, allow overage but flag `in_overage=true`. | ☐ Integration test spins 201 h worth of small instances and sees overage flag in Polar *and* API response.<br>☐ `orz cloud list` CLI shows “Overage” badge once quota exceeded. |
| 6 | **Budget alerts (50 %, 90 %)** | • New async job checks usage daily.<br>• Send email via existing notifier service at 50 % and 90 % of included hours. | ☐ Fake SMTP test captures 2 emails at the correct thresholds.<br>☐ Emails contain org name, hours used, reset date, and manage‑plan link. |
| 7 | **Idle throttling & daily cap** | • Re‑use existing idle‑reaper; add per‑org **max 8 h continuous** & **24 h daily** caps.<br>• Exceeding cap auto‑pauses the runner. | ☐ End‑to‑end test spins instance >8 h – control plane transitions it to **Paused (cap)**.<br>☐ Prometheus counter `orzbob_idle_instances_reaped_total` increments. |
| 8 | **CLI & dashboard surfacing** | • `orz cloud billing` shows:<br> • plan, hours used, reset date, estimated next bill.<br>• Landing page dashboard card with the same data. | ☐ `orz cloud billing --json` returns machine‑readable object.<br>☐ Storybook snapshot for dashboard card passes CI. |
| 9 | **Docs & support playbooks** | • Update **docs/pricing.md** & landing page.<br>• Add runbook “Rectify incorrect charges”. | ☐ Pull request reviewed by product & support.<br>☐ Docs site redeployed; heading “Starts at **$20/mo** incl. 200 h”. |

---

## How to use

1. Work on one checkpoint at a time; open a *Draft PR* referencing the checkpoint number.  
2. When **Done when** items pass locally & in CI, mark the checklist item complete in the PR description.  
3. Merge once code review & QA sign‑off.

> **Tip:** run `make launch-readiness` before releasing to guarantee all billing gates are compiled into the readiness script.

Happy shipping! 🚀
