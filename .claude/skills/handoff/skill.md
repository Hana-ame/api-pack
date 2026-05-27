---
name: handoff
description: Compacts the current conversation into a handoff document so another agent can continue the work without losing context.
---

Compress the current conversation into a handoff document. Include:

- What was being worked on and why
- Key decisions made and their rationale
- Current state (what's done, what's in progress, what's blocked)
- Files changed and their purpose
- Next steps (ordered by priority)
- Any open questions or unknowns

Write the result to `docs/handoff.md`. Keep it concise — another agent should be able to pick up from here without reading the full conversation.
