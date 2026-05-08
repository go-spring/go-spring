---
name: doc-writer
description: Use when writing, rewriting, reviewing, or expanding Go-Spring documentation. Applies the Go-Spring documentation style, verifies facts against source code/tests/examples, and keeps docs consistent with the existing docs structure.
---

# Go-Spring Doc Writer

Use this skill for Go-Spring documentation work, including new docs, rewrites, reviews, structure changes, terminology cleanup, examples, FAQs, and integration guides.

## Workflow

1. Identify the document type: overview, getting started, guide, integration, example, FAQ, changelog, or contributor doc.
2. Verify factual claims against source code, tests, examples, or existing docs before writing API names, configuration keys, defaults, behavior, or version/status notes.
3. Read `references/writing-style.md` when style, structure, terminology, examples, cross references, or visual conventions matter.
4. Preserve the role of `docs/` as reader-facing documentation. Keep AI role material, authoring rules, checklists, and templates under `skills/doc-writer/`.
5. Write in Chinese unless the target document is already English or the user asks otherwise.
6. Before finishing, check that examples have enough context, terms are consistent, links are relevant, and limits or edge cases are explicit.

## Reference Files

- `references/writing-style.md`: detailed Go-Spring documentation writing rules, rule levels, terminology, examples, and visual conventions.
