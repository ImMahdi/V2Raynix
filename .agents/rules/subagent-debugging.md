# Rule: Systematic Subagent Debugging

## Protocol
1. **Reproduce First:** Always write or run an automated test that reliably reproduces the bug or failure.
2. **Isolate Root Cause:** Never apply speculative patches. Inspect logs, stack traces, and exact failure states.
3. **Minimal Fix:** Change only what is necessary to resolve the root cause while maintaining backward compatibility.
4. **Verify Regressions:** Run the full package test suite to ensure no side-effects or regressions were introduced.
