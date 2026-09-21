# Domain Briefing 06: Atomic Store & Persistence Subsystem

> **Subagent Role:** Principal Storage Systems & Concurrency Auditor  
> **Audited Files:** `internal/store/store.go`, `internal/store/store_test.go`  
> **Output Report File:** `docs/audit/reports/06-store-report.md`  
> **Mode:** STRICTLY READ-ONLY AUDIT. DO NOT MODIFY ANY SOURCE CODE.

---

## 1. Deep Knowledge Transfer & Architecture

The `store` package provides file-backed persistence for proxy configurations, active routing rules, administrator credentials, and system settings.

### Key Architectural Concepts:
1. **Store Interface & Data Structures:**
   - Defines `Store` interface covering configs, rules, user accounts, and system settings.
   - `fileStoreData`: Root container serializing maps of `Configs`, `RoutingRules`, `AdminUser`, and `Settings`.
2. **Atomic Write Pattern (`persist`):**
   - Serializes state with `json.MarshalIndent(fs.data, "", "  ")`.
   - Writes to a temporary file: `tmpFile := fs.filePath + ".tmp"` with permissions `0600`.
   - Replaces the target file using POSIX atomic rename: `os.Rename(tmpFile, fs.filePath)`.
3. **Thread Safety:**
   - Uses `fs.mu sync.RWMutex` to protect read access (`GetConfigs`, `GetConfigByID`) and write access (`SaveConfig`, `DeleteConfig`, `SetActiveConfig`).
   - Mutations immediately trigger synchronous `fs.persist()` under lock.

---

## 2. Concerns, Pitfalls & Critical Warnings

When auditing this domain, pay hyper-vigilant attention to these failure modes:
1. **Missing `fsync()` Flush Hazard (Zero-Byte File on Crash):**
   In `persist()`:
   ```go
   if err := os.WriteFile(tmpFile, bytes, 0600); err != nil { ... }
   if err := os.Rename(tmpFile, fs.filePath); err != nil { ... }
   ```
   `os.WriteFile` writes data to the OS page cache and closes the file WITHOUT calling `f.Sync()`. On Linux filesystems (ext4, xfs), if the system loses power or reboots shortly after, the directory entry updates to point to an empty, unflushed inode. Without `f.Sync()` on both file and parent directory, data loss or corruption is possible.
2. **Missing Backup / Auto-Recovery Mechanism:**
   If `fs.load()` encounters a corrupted JSON file (e.g. from an incomplete write or storage glitch), it fails immediately with `corrupted store json: %w`, preventing the entire V2Raynix service from booting. There is no automated fallback to a `.bak` snapshot.
3. **Disk I/O Bottlenecks Under Write Mutex:**
   `fs.persist()` runs synchronously while `fs.mu.Lock()` is held. During batch operations (e.g. importing 500 proxy configs or updating latency for 100 nodes in `pinger`), disk I/O serialization blocks all readers and writers.
4. **Shallow Copy vs Pointer Integrity:**
   `GetConfigs()` copies `*cfg` as `itemCopy := *cfg`, returning pointers to new structs. However, if any internal fields in future revisions are reference types or maps, shallow copies may allow caller mutations to bleed into store state.
5. **Windows File Locking Hazard:**
   On Windows platforms, `os.Rename(tmpFile, fs.filePath)` fails with `Access is denied` if another process (antivirus, backup daemon, or indexing service) has a read lock on the target file.

---

## 3. Lead Architect Directives & Step-by-Step Instructions

### Step 1: Mandatory Architectural Graph Exploration (`graphify`)
Before reading line-by-line, run:
```powershell
python -m graphify.cli explain store
# or
python -m graphify.cli query "how does store interact with api, core, and configmgr"
```
Understand all call sites invoking `SaveConfig`, `UpdateLatency`, and `SaveRoutingRule`.

### Step 2: Initialize Report Scaffold
Immediately create `docs/audit/reports/06-store-report.md` with standard sections.

### Step 3: Step-by-Step Systematic Code Inspection & Incremental Updates
Audit each area sequentially and update the report immediately:
1. **Persistence & Fsync Reliability (`store.go:L112-L128`):** Audit atomic file write, directory flushing, and durability.
2. **Corruption Recovery & Deserialization (`store.go:L83-L111`):** Audit error handling on malformed JSON and missing fields.
3. **Concurrency Locking & Contention (`store.go:L130-L288`):** Audit mutex scope, RLock vs Lock usage, and latency during batch operations.
4. **Data Isolation & Deep Copying (`store.go:L130-L155`):** Audit whether returned structs are fully decoupled from store internal state.

### Step 4: Strict Adherence to 7-Field Defect Schema
Every finding in the report must include:
1. `Title & Category`
2. `Code Location` (`Lxx-Lyy`)
3. `Severity` (`Critical`, `High`, `Medium`, `Low / Refactor`)
4. `Trigger Scenario & Root Cause Analysis`
5. `Proof of Concept / Verification Method`
6. `Recommended Architectural Fix`
7. `Existing Strengths & Robustness`

**REMINDER:** Strictly read-only. Do not edit source code. Stay 100% focused on your assigned domain.
