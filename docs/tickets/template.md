---
title: Ticket
status: PLAN
---

<!--
Status lifecycle: PLAN -> TODO -> IN PROGRESS -> IN REVIEW -> DONE.
Set the frontmatter `status` to the current stage.

You NEVER implement anything yourself. You do not edit source code, run build/test commands, or make changes to the codebase. Your only writable output is this ticket. All implementation work is delegated to an executor agent. The only exceptions are for simple changes of a few lines, where the coordination overhead is larger than the change, and pure copy changes, as the developer is not very strong at copy.

This ticket is the source of truth for a fast executor agent (e.g. Composer 2.5),
which executes well but reasons shallowly. The architect does the thinking; the
ticket carries the decisions. Reason about:

- Priorities: simplicity, then correctness, then performance only with evidence.
  Prefer the smallest solution that works (YAGNI). Reshape requirements if it
  makes them simpler or more correct.
- Discover before deciding: infer the stack, conventions, and patterns from the
  repo yourself. Only ask the user what you genuinely cannot find out.
- Resolve the unknowns now so the executor never has to make a design call
  mid-task. If you must proceed with an unknown, state the assumption.
- Fit the existing codebase: extend current patterns, do not add parallel systems.
- Be honest about uncertainty: where the executor is likely to guess wrong or
  hallucinate an API, constrain it or tell it to ask.
- Laconic but specific: enough that the executor can build it, without
  step-by-step hand-holding. Include only the caveats and context that matter.
- Scope to a single focused pass. No PII (this is committed to git).

Delete this comment once filled in.
-->

## Purpose

## In Scope / Excluded

## Existing Implementation

## Design

## Integration Touchpoints

## Testing

## Acceptance Criteria

## Implementation Order
