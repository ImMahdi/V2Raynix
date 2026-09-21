# Domain Briefing 05: Auth & Session Security Subsystem

> **Subagent Role:** Principal Cryptographic Security & Authentication Systems Auditor  
> **Audited Files:** `internal/auth/auth.go`, `internal/auth/auth_test.go`  
> **Output Report File:** `docs/audit/reports/05-auth-security-report.md`  
> **Mode:** STRICTLY READ-ONLY AUDIT. DO NOT MODIFY ANY SOURCE CODE.

---

## 1. Deep Knowledge Transfer & Architecture

The `auth` package implements password security and stateless JSON Web Token (JWT) session generation and validation to protect V2Raynix REST endpoints.

### Key Architectural Concepts:
1. **Password Hashing:**
   - Uses `golang.org/x/crypto/bcrypt` with `bcrypt.DefaultCost` (work factor 10).
   - Functions: `HashPassword(password string)` and `CheckPassword(hashedPassword, password string)`.
2. **JWT Construction (`GenerateJWT`):**
   - Implements custom HMAC-SHA256 (`HS256`) token construction without heavy third-party dependencies.
   - Header: `{"alg":"HS256","typ":"JWT"}` encoded with `base64.RawURLEncoding`.
   - Payload (`Claims`): `Username`, `IssuedAt` (`iat`), `ExpiresAt` (`exp`).
   - Signature: `HMAC-SHA256(headerB64 + "." + claimsB64, secret)`.
3. **JWT Verification (`ValidateJWT`):**
   - Splits token into 3 parts by `"."`.
   - Computes expected signature using server's HMAC secret and compares using constant-time `hmac.Equal(sig, expectedSig)`.
   - Unmarshals claims and verifies expiration against current Unix timestamp: `time.Now().Unix() > claims.ExpiresAt`.

---

## 2. Concerns, Pitfalls & Critical Warnings

When auditing this domain, pay hyper-vigilant attention to these failure modes:
1. **Empty Secret Key & Weak Secret Handling:**
   What happens if `secret` passed to `GenerateJWT` or `ValidateJWT` is empty (`len(secret) == 0`) or too short? If the server starts without a configured secret or falls back to an empty byte slice, HMAC with an empty key allows arbitrary token generation and forgery. There is no minimum entropy check (e.g. at least 32 bytes).
2. **Clock Skew & Replay Window Vulnerabilities:**
   `time.Now().Unix() > claims.ExpiresAt` performs a strict, zero-tolerance equality check with no clock skew tolerance (e.g. 1-2 minutes). If the server clock drifts or synchs via NTP during a session, valid tokens may suddenly be rejected or expired tokens accepted.
3. **Missing Header Validation in `ValidateJWT`:**
   `ValidateJWT` recalculates the HMAC using the server's hardcoded algorithm and secret, but it completely discards and never parses `parts[0]` (the header). Does it verify `typ: "JWT"` or ensure the algorithm isn't tampered with?
4. **Lack of Token Revocation / Blacklist:**
   V2Raynix issues 24-hour JWT tokens. If an admin changes their password, or logs out, previously issued JWTs remain cryptographically valid until the 24-hour window lapses. Can an attacker who steals a token continue accessing root network endpoints after password change?
5. **Bcrypt Default Cost Future-Proofing:**
   `bcrypt.DefaultCost` is 10. While standard, does it provide adequate protection against offline GPU brute-forcing on modern hardware? Is the cost configurable?

---

## 3. Lead Architect Directives & Step-by-Step Instructions

### Step 1: Mandatory Architectural Graph Exploration (`graphify`)
Before reading line-by-line, run:
```powershell
python -m graphify.cli explain auth
# or
python -m graphify.cli query "how does auth package integrate with api and store"
```
Trace where `JWTSecret` originates (in `cmd/server/main.go` or `internal/api/router.go`).

### Step 2: Initialize Report Scaffold
Immediately create `docs/audit/reports/05-auth-security-report.md` with standard sections.

### Step 3: Step-by-Step Systematic Code Inspection & Incremental Updates
Audit each area sequentially and update the report immediately:
1. **Password Hashing Security (`auth.go:L32-L46`):** Check bcrypt cost, error checking, and timing leak resistance.
2. **JWT Generation & Claims (`auth.go:L47-L80`):** Audit entropy, timestamp handling, base64 encoding scheme, and signature generation.
3. **JWT Validation Mechanics (`auth.go:L81-L122`):** Audit constant-time comparison, malformed token handling, expiration checks, and header validation.
4. **Secret Lifecycle & Initialization:** Inspect how the secret key is provided and handled if empty.

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
