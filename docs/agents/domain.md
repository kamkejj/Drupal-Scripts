# Domain Docs

How the engineering skills should consume this repo's domain documentation when exploring the codebase.

## Before exploring, read these

- `CONTEXT.md` at the repo root
- `CONTEXT-MAP.md` at the repo root if it exists, then each relevant context it references
- Relevant ADRs under `docs/adr/`

If these files do not exist, proceed silently. Domain documentation is created lazily when terms or decisions are resolved.

## File structure

This is a single-context repository. Domain terminology belongs in the root `CONTEXT.md`, and architectural decisions belong under `docs/adr/`.

## Use the glossary's vocabulary

When output names a domain concept, use the term defined in `CONTEXT.md`. Do not drift to synonyms the glossary explicitly avoids.

If a needed concept is absent, reconsider whether the language fits the project or note the gap for a future documentation session.

## Flag ADR conflicts

Surface any conflict with an existing ADR explicitly rather than silently overriding the decision.
