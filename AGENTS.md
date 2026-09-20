# Project Rules & Development Workflow: V2Raynix

## Core Methodology: Superpowers & Subagent-Driven Development

All AI agents working on this project MUST strictly follow the **Superpowers** workflow principles. Under no circumstances should an agent write unverified code, make unconfirmed assumptions, or bypass the structured development lifecycle.

---

### 1. Mandatory Skill Invocation (`using-superpowers`)
Before executing any action, code modification, or answering complex questions, invoke the appropriate Superpowers skill:
- **New Features & Architectural Decisions:** MUST start with `superpowers:brainstorming`. Do not write code or finalize plans until user intent, scope, and edge cases are clarified.
- **Planning:** Multi-step work MUST produce a formal, granular plan with `superpowers:writing-plans` (specifying exact files, steps, test criteria, and verification commands).
- **Execution & Subagents:** Use `superpowers:subagent-driven-development` to dispatch fresh implementer subagents per task and review subagents after each task. For independent domain tasks or concurrent bugfixes, dispatch parallel agents with `superpowers:dispatching-parallel-agents`.
- **Testing (TDD):** Adhere to `superpowers:test-driven-development` — write failing tests first (Red), write minimal code to pass (Green), then refactor.
- **Debugging:** Never guess or patch blindly. Follow `superpowers:systematic-debugging` to isolate root causes.
- **Verification:** Follow `superpowers:verification-before-completion`. Never assert success without running the actual test/verification commands and showing real output.
- **Code Review:** Follow `superpowers:requesting-code-review` before finalizing or merging branches.

---

### 2. Subagent Architecture & Context Hygiene
- The primary agent functions as the **Lead Architect & Controller**. It maintains the high-level roadmap, decisions ledger, and user communication.
- Dispatch **fresh subagents** for individual implementation tasks to prevent context pollution and memory degradation.
- Each implementer subagent receives only its specific task brief, interfaces, and constraints.
- Each completed task must be verified by a reviewer subagent (checking both spec compliance and code quality) before marking complete.

---

### 3. Task Tracking & Transparency
- Maintain a live task artifact/ledger detailing every step, commit, and decision (`Ruling`).
- Keep the user informed with clear, evidence-backed status updates instead of speculative assertions.
- When trade-offs or ambiguities arise, record explicit rulings and get user sign-off when appropriate.
