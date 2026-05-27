---
name: improve-codebase-architecture
description: Find deepening opportunities in the codebase. Identifies code smells, architectural weaknesses, and suggests concrete improvements guided by domain language and design decisions.
---

Scan the codebase for architectural improvement opportunities:

1. **Domain alignment** — Does the code structure reflect the domain language? Are names consistent with how the business thinks about the problem?
2. **Coupling** — Are there modules that know too much about each other? Can dependencies be inverted?
3. **Cohesion** — Are there modules doing too many unrelated things? Can they be split?
4. **Abstraction level** — Is each function/class at a consistent level of abstraction? Are there functions mixing high-level policy with low-level details?
5. **Duplication** — Is there duplicated logic that should be extracted?

For each finding, explain: what's wrong, why it matters, and a concrete suggestion for improvement. Present findings one at a time, from most to least impactful.
