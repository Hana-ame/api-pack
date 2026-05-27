---
name: tdd
description: Test-driven development with red-green-refactor loop. Builds features or fixes bugs one vertical slice at a time, writing tests first.
---

Follow a strict red-green-refactor loop:

1. **Red** — Write a failing test that defines the expected behavior
2. **Green** — Write the minimum code to make the test pass
3. **Refactor** — Clean up the code while keeping tests green

Build features one vertical slice at a time. Each slice should:
- Be independently testable
- Deliver a complete (if minimal) user-visible behavior change
- Pass all existing tests before moving to the next slice

Never write implementation code before the test. Never skip the refactor step.
