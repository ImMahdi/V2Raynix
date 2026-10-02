# Rule: Superpowers & Subagent Workflow

## Always Follow Superpowers
1. **Brainstorming Before Implementation:** When adding features or changing behaviors, always invoke `superpowers:brainstorming` first. Ask clarifying questions one at a time.
2. **Implementation Planning:** Create structured, bite-sized tasks using `superpowers:writing-plans`.
3. **Subagent Execution:** Follow `superpowers:subagent-driven-development`. Use fresh subagents for implementation tasks to avoid context pollution, followed by task-reviewer subagents.
4. **Test-Driven Development:** Write failing tests first before writing implementation code (`superpowers:test-driven-development`).
5. **Systematic Debugging:** In case of errors or unexpected behavior, follow `subagent-debugging.md` and `superpowers:systematic-debugging`. Do not guess.
6. **Strict Verification:** Never claim a task is complete without running verification commands and observing real output (`superpowers:verification-before-completion`).
