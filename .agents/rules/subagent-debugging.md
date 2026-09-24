# Rule: Autonomous Subagent Debugging with `agy`

## Core Principle
Whenever an error, test failure, bug report, or unexpected behavior arises, never guess or patch blindly. Follow this autonomous 4-phase debugging lifecycle using `agy` subagents and file-based state transfer (`docs/debug/`).

---

## 1. Subagent Dispatch Pattern (Windows / PowerShell)
Always use non-interactive piped execution with permission bypass so the subagent runs unattended without blocking on stdin/TTY:

```powershell
$null | agy --add-dir "c:\MaadZone\Github Projects\V2Raynix" `
            --model gemini-3.8-flash-medium `
            --effort medium `
            --dangerously-skip-permissions `
            -p "<Concise Task Prompt pointing to Brief and Report files>"
```

### Parallel Execution (PowerShell Background Jobs)
When investigating multiple independent domains or concurrency issues simultaneously:
```powershell
$job = Start-Job -ScriptBlock {
    param($ws, $pr)
    $null | agy --add-dir $ws --model gemini-3.8-flash-medium --effort medium --dangerously-skip-permissions -p $pr
} -ArgumentList $workspace, $prompt
```
Monitor job progress via `$job.State` and report file size on disk, then retrieve outputs with `Receive-Job`.

---

## 2. File-Based State Transfer (Brief & Report)
Avoid relying on transient terminal stdout. Communication between the Lead Architect and subagents must be persisted on disk:
- **Task Brief (`docs/debug/briefs/<id>-brief.md`):** Prepared by the Lead Architect. Contains error symptoms, log traces, affected modules, architectural constraints, and target goals.
- **Findings Report (`docs/debug/reports/<id>-report.md`):** Progressively written by the subagent. Contains root cause analysis, stack traces, code line references (`Lxx-Lyy`), and remediation steps.

---

## 3. Four-Phase Debugging Lifecycle

### Phase 1: Investigation Subagent (Strictly Read-Only)
- **Role:** Deep forensic analysis without side effects.
- **Constraint:** Strictly READ-ONLY on existing source code.
- **Workflow:**
  1. Read briefing from `docs/debug/briefs/<id>-brief.md`.
  2. Query `graphify` (`python -m graphify.cli explain <subsystem>`) for topological and architectural dependencies.
  3. Trace logs, error flows, and recent diffs.
  4. Write verified root-cause analysis to `docs/debug/reports/<id>-report.md`.

### Phase 2: Reproduction Test (RED)
- **Role:** Establish deterministic proof before modifying source code (TDD).
- **Workflow:**
  1. Write an isolated unit or integration test reproducing the exact failure state.
  2. Execute the test and verify it fails with the exact reported error.

### Phase 3: Targeted Remediation Subagent (GREEN)
- **Role:** Minimal, precise code fix.
- **Workflow:**
  1. Review the root cause report and reproduction test.
  2. Implement the minimal necessary fix to satisfy the test.
  3. Execute reproduction test and confirm it passes (GREEN).

### Phase 4: Regression & Live System Verification
- **Role:** Complete verification before closing the issue.
- **Workflow:**
  1. Run the entire test suite across all packages (`go test ./...`).
  2. If web frontend is affected, run frontend test / lint / build.
  3. If relevant to daemon/service, verify against the live target server (`192.168.254.80:2080`).
