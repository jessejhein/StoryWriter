# 00 — Project Brief

## Product thesis

**AI Story Workshop** is a local-first writing environment for authors who want controllable AI assistance without surrendering canon, workflow, or model choice.

The product helps the author:

1. import messy notes,
2. extract story structure,
3. build a Codex,
4. write and revise scenes,
5. experiment with what-if branches,
6. compare consequences,
7. selectively regenerate only affected material,
8. use different agents/models for different jobs.

## Primary audience

Single authors on a budget:

- hobbyists,
- students,
- retirees,
- tinkerers,
- writers who want to run some models locally,
- writers who want to experiment with many agents/styles rather than pay for one locked-in service.

## Product principles

### 1. Human canon is sacred

The human author decides what is true.

AI may produce:

- suggestions,
- patches,
- draft alternatives,
- structured proposals.

AI must not silently change canonical story state.

### 2. Use the least context needed

Do not dump the whole Codex into every prompt.

Paragraph polish may need only selected text, nearby paragraphs, and a voice card.

Chapter refinement may need active Codex entries, chapter text, outline neighbors, and progressions.

Outline work may need global story state.

### 3. Agents/styles are core, not decoration

Different models do different jobs. A small 13B/8k model may be poor at outlining but excellent at line-level wording. The app should make that useful.

### 4. Branching is a product feature

The user should be able to ask:

```text
What if we did this instead?
```

The app should create a branch, analyze ramifications, generate affected alternatives, compare, and let the author promote or discard.

### 5. Local-first and portable

A story project should be a folder the user can understand, back up, put in Git, or inspect manually.

## MVP success criteria

The MVP is complete when a single author can:

1. create/open a local project folder,
2. import Markdown notes,
3. extract candidate Codex entries and outline items into a review queue,
4. approve/edit/merge those candidates,
5. edit scenes in a Vim-friendly web editor,
6. use timeline-aware Codex context for AI actions,
7. run paragraph/chapter actions through selectable agents/styles,
8. create a basic what-if Git branch,
9. compare branch changes to canon,
10. promote selected changes manually.

## Not the MVP

Do not implement these in MVP unless explicitly asked:

- real-time collaboration,
- mobile polish,
- DOCX/Scrivener/EPUB export,
- full prompt marketplace,
- MCP compatibility,
- rich graph visualizations,
- full chat-first interface,
- hosted SaaS account/billing system,
- auto-publishing,
- elaborate WYSIWYG formatting.

