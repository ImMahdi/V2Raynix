# Domain Briefing 05: Auth, Security & Atomic Store Persistence Subsystem

> **Subagent Role:** Principal Cryptography, Session Security & Atomic Storage Auditor  
> **Audited Files:** `internal/auth/auth.go`, `internal/auth/*_test.go`, `internal/store/store.go`, `internal/store/*_test.go`  
> **Output Report File:** `docs/audit/reports/05-auth-store-report.md`  
> **Mode:** STRICTLY READ-ONLY AUDIT. DO NOT MODIFY ANY SOURCE CODE.

---

## 1. Subsystem Architecture & Responsibilities
This domain secures administrative access and guarantees atomic persistence:
- `internal/auth`: Handles password hashing (`bcrypt`), JWT token issuance, claims validation, and authentication verification.
- `internal/store`: Manages thread-safe JSON-backed persistence for proxy configurations, routing rules, user credentials, and system settings via atomic file replacement (`.tmp` -> sync -> rename).

---

## 2. Critical Audit Directives & Failure Modes to Investigate

Auditor must systematically examine:
1. **Password Hashing & Timing Attacks:**
   - Does password verification use constant-time comparisons (`subtle.ConstantTimeCompare`)?
   - Is bcrypt cost appropriate (not under-costed or vulnerable to DoS)?
2. **JWT Token Lifespan, Signature Verification & Revocation:**
   - Are algorithm and signature verified strictly? Does it defend against `alg: none` or HMAC/RSA confusion?
   - Does token revocation exist when an admin password is reset or changed?
3. **Atomic File Persistence & Data Loss Hazards (`store.go`):**
   - Is `fsync()` called on the temporary file before atomic rename (`os.Rename`) to prevent zero-byte files on power loss?
   - On Windows environments, does `os.Rename` handle file locking conflicts gracefully?
   - Is there backup / automatic recovery if the JSON file is corrupted?
4. **Mutex Granularity & I/O Contention:**
   - Is disk I/O performed while holding the master read/write mutex, causing request blocking during file write operations?
5. **Shallow Copy vs Pointer Integrity:**
   - Do query methods (`GetConfigs`, `GetSettings`) return deep copies, or do callers receive pointers allowing concurrent mutation of internal state?

---

## 3. Subagent Execution Instructions
1. Run `python -m graphify.cli explain auth` and `python -m graphify.cli explain store` or query graph to understand dependencies.
2. Progressively write findings to `docs/audit/reports/05-auth-store-report.md`.
3. Every finding MUST follow the 7-field defect schema (ID, Code Location, Severity, Trigger/Root Cause, PoC, Recommended Fix, Strengths).
4. STRICTLY READ-ONLY: Do not edit any code files.
