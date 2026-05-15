# Fyne Writer

An AI-powered Markdown writing tool with a custom WYSIWYG editor, built
in Go on the [Fyne](https://github.com/fyne-io/fyne) toolkit.

**Resuming development?** Read this README top-to-bottom — it's the
source of truth. Sections most relevant to picking up work:

- [Current state — what works today](#current-state--what-works-today) — feature inventory.
- [Not yet built (roadmap)](#not-yet-built-roadmap) — M5 Phase B (comments) is up next.
- [Architectural conventions](#architectural-conventions) — load-bearing patterns to follow.
- [M5 design notes](#m5-design-notes) — version history shipped; comments still designed.
- [Why we chose what we chose](#why-we-chose-what-we-chose) — context behind the decisions.
- `git log --oneline` — commit messages are detailed; each milestone is its own commit.

## Run

Requires Go 1.22+ and an API key from at least one provider.

```sh
cp .env.example .env
# fill in ANTHROPIC_API_KEY and/or OPENAI_API_KEY
go build -o fyne-writer ./cmd/fyne-writer
./fyne-writer
```

`./fyne-writer --check-ai` runs a one-shot prompt against the active
provider and prints the reply — handy for verifying the .env wiring.

If no key is present the app falls back to a `mock` provider that echoes
input; the GUI still runs.

## Current state — what works today

The custom rich-text editor, the flagship AI layer, AI document checks,
custom prompts, the Settings dialog, and per-save version history (M0–M5
Phase A in the internal milestone plan) are shipped. Comments and the
read-only-URL publishing piece are not yet implemented.

### Editor surface

| Feature | Status | Notes |
|---|---|---|
| Type, click-to-position caret, arrow nav, Home/End | ✓ | |
| Mouse drag-select, shift+arrow select, cmd+A | ✓ | |
| Cut / copy / paste (cmd+X / cmd+C / cmd+V) | ✓ | Markdown round-trip on paste |
| Undo / redo (cmd+Z / cmd+shift+Z) | ✓ | Typing runs coalesce; caret motion ends the run |
| Bold / italic / underline / code / strike marks | ✓ | cmd+B / cmd+I / cmd+U / cmd+E / cmd+shift+X |
| Headings H1–H6 | ✓ | cmd+1 / cmd+2 / cmd+3 toggle; backspace at start reverts to paragraph |
| Bullet + ordered lists | ✓ | Round-trip via `Meta{list_kind, depth, index}` |
| Blockquote | ✓ | Renders with muted left bar |
| Fenced code block | ✓ | Monospace + background fill |
| Horizontal rule | ✓ | Renders as a thin line |
| Right-click context menu | ✓ | Cut/Copy/Paste/Select All + Format submenu + AI items |
| File menu shortcuts | ✓ | cmd+N / cmd+O / cmd+S / cmd+shift+S |
| Markdown round-trip (open/save .md) | ✓ | All block + mark types preserved byte-for-byte where feasible |
| YAML front-matter for doc metadata | ✓ | Stores per-doc background instructions |

### AI layer

| Feature | Status | Notes |
|---|---|---|
| Anthropic provider (anthropic-sdk-go) | ✓ | Honors per-command temperature |
| OpenAI provider (openai-go) | ✓ | Temperature unset (gpt-5 only accepts default) |
| Mock provider fallback | ✓ | No key → app still runs |
| Provider auto-pick from .env | ✓ | Anthropic preferred when both keys present |
| Streaming AI replacement into selection | ✓ | One undo step regardless of chunk count |
| Command palette (cmd+K) | ✓ | Modal popup with filterable list |
| **Paraphrase** on selection | ✓ | Right-click or palette |
| **Shorten** on selection | ✓ | Right-click or palette |
| **Expand** on selection | ✓ | Right-click or palette |
| **Fix tone (formal)** on selection | ✓ | Right-click or palette |
| **Context-aware Synonyms** for a single word | ✓ | Right-click word → AI returns up to 8 synonyms that fit the sentence |
| Per-document **Background Instructions** | ✓ | AI menu → Background Instructions; stored as YAML front-matter |
| **Document checks** (grammar / clarity / style) | ✓ | cmd+shift+K or sidebar; wavy underline + Accept/Reject panel |
| **Custom prompts library** | ✓ | AI menu → Prompts Library; text/template variables, optional hotkey, palette + right-click integration |
| **Settings dialog** | ✓ | cmd+, ; provider + per-provider model + light/dark/system theme; persisted in SQLite |

### Storage

| Feature | Status | Notes |
|---|---|---|
| SQLite store with WAL + foreign keys | ✓ | At `~/Library/Application Support/fyne-writer/fyne-writer.db` on macOS |
| Schema for settings / versions / comments / prompts | ✓ | Tables exist; comments UI lands in M5 Phase B |
| **Version snapshots on save** | ✓ | Versions tab in sidebar; unified diff preview; Restore is one undo step. Dedup on identical content hash. Capped at 50 per doc. |

## Not yet built (roadmap)

In rough order:

- **M5 Phase B — Comments.** Anchor free-text comments to byte ranges;
  sidebar tab; right-click → "Add comment...". The Versions piece of
  M5 is already shipped (see Current state). Reuse the AI-issues
  anchor-resolution pattern; consider lifting `editor.Issue` rendering
  into a generic `editor.Annotation` so comments + issues share the
  wavy-underline pool.
- **M6 — Polish.** File-browser sidebar, status bar, keyboard shortcut
  cheatsheet, first-run onboarding, Inter font embedding, performance
  pass on long documents. Also: replace Fyne's built-in file dialogs
  with custom modals that handle Esc, and upgrade the issue underline
  from straight zigzag to a smoother wavy curve.
- **Deferred to v2 — read-only URL publishing.** Requires a backend
  service (out of scope for v1).

## Known limitations

- **No font embedding yet.** UI falls back to Fyne's default fonts.
  Inter + JetBrains Mono will be embedded in M6 polish.
- **Word wrap measures at the base style.** Heading and code lines may
  overflow the right edge by a few pixels because wrap uses the plain
  body metric. Acceptable for now; visible only at narrow widths.
- **No real-time mark indicator in the status bar.** Pressing cmd+B
  with a collapsed caret sets a pending-marks state but there's no
  visible feedback until you type a character.
- **Lists are flattened during edit.** Nested lists round-trip through
  markdown via `Meta{depth}` but the editor doesn't yet support
  Tab / Shift+Tab to change depth, or Enter-on-empty-item to exit.
- **OpenAI temperature is fixed at the API default.** gpt-5 only
  accepts the default; we drop per-command temperatures rather than
  branching by model. Anthropic still honors per-command temperatures.
- **Synonyms picker is a popup, not an inline submenu.** The AI call
  is async, so we use a follow-up dialog rather than a sub-menu that
  would have to populate after the parent menu closed.
- **Esc doesn't close Fyne's built-in file dialogs (cmd+O / cmd+S).**
  Fyne's `dialog.FileDialog` focuses an internal `widget.Entry` that
  silently consumes Esc, and Fyne's widget-bypassing hooks
  (`canvas.SetOnTypedKey`, `canvas.AddShortcut`) either require no
  focused widget or require a modifier — Esc-without-modifier never
  becomes a `Shortcut`. Workaround for now: click the "Cancel" button.
  Proper fix is a custom file picker that uses our `escEntry` pattern;
  queued for M6 polish. Our own modal popups (cmd+K, Synonyms, etc.)
  already handle Esc correctly.
- **Document check sends the whole document as one chunk.** Truncated
  to ~6k chars, so longer documents get a partial scan. Paragraph
  chunking is deferred until the limit is hit in practice.
- **Issue anchors must be unique within a single block.** Suggestions
  whose anchor text appears more than once (within the same block or
  across blocks) are silently dropped at resolution time, because
  LLM-returned anchors can't reliably disambiguate. The system prompt
  asks for ≥6-char anchors with surrounding context to reduce this.
- **Editing the document drops invalidated issues.** Any issue whose
  anchor text no longer matches at the recorded offset is removed
  rather than re-resolved. Re-run the check after editing.
- **Issue underline is a zigzag, not a true wave.** Built from short
  `canvas.Line` segments. Visually adequate; upgrade to a smoother
  curve is M6 polish.
- **Custom-prompt hotkeys are limited to single letter / digit.** The
  parser accepts `Cmd|Ctrl|Shift|Alt` modifiers plus one A–Z or 0–9
  key. No function keys or punctuation yet.

## Architectural conventions

Patterns to follow when extending this codebase. Most aren't obvious from
the code alone.

### Document model — `internal/doc`

- **`Position.Offset` is byte offset within the block's concatenated
  `PlainText()`, not within a specific inline.** `Position.Inline` is
  vestigial (always 0). The inline structure inside a block is purely
  about marks; whenever we need to know which inline owns a byte, we
  use `doc.InlineAt(b, byteOff)`. This kept the editor math simple
  during the M2b multi-inline refactor.
- **Block-text mutations always go through `doc.InsertText`,
  `doc.DeleteRange`, `doc.ApplyMark`, `doc.SplitBlock`.** They preserve
  inline boundaries and normalize the inline list (merging adjacent
  inlines with matching marks, dropping empty inlines). Never mutate
  `Block.Inlines` directly outside of `internal/doc`.
- **Lists are flattened.** Bullet/ordered lists do not contain
  `Children`; each list item is a top-level `BlockListItem` carrying
  `Meta{list_kind, depth, index}`. The markdown writer regroups
  consecutive items back into a list on save. This kept editor path
  math single-element.
- **Front-matter is YAML at the top of `.md`, parsed/written via
  `doc/frontmatter.go`.** Currently only `instructions:` (background
  instructions for AI). Add new fields to `DocMeta` + the
  `parseMeta`/`writeMeta` pair.

### Editor — `internal/editor`

- **Every mutation calls `e.commitUndo(kind)` before mutating.**
  Consecutive same-kind operations (typing, deletion) coalesce into
  one undo entry. Caret motion or any "other" mutation breaks the run.
  The undo stack is a ring of `*doc.Document` clones — cheap given
  document size, eliminates inverse-op bookkeeping.
- **Renderer holds pools of canvas objects keyed by purpose:**
  `textObjs` (one per styled run), `selRects`, `deco` (underline /
  strike lines), `gutterText` (list bullets), `blockBars` (quote left
  bar), `blockBGs` (code block fill), `hrLines`. Sync functions
  resize the pool and hide unused entries off-screen.
- **Word-wrap measures with the block's base style only.** Bold/italic
  metric drift across a wrapped line is accepted as a known limitation
  (see [Known limitations](#known-limitations)).
- **Modifier-key suppression in `TypedRune`.** Fyne's glfw driver
  delivers a char event AND a shortcut for `cmd+letter` combos on
  macOS. We track cmd/ctrl/alt via `desktop.Keyable` `KeyDown`/`KeyUp`
  and ignore `TypedRune` when any is held — otherwise the editor
  would insert the letter, overwriting the selection right before the
  shortcut runs.
- **Threading: `fyne.Do` for cross-thread UI updates.** The caret blink
  goroutine and AI streaming goroutines both marshal their renderer/
  canvas changes through `fyne.Do(func() { ... })`. Fyne logs noisy
  thread-safety warnings if you forget.
- **`SetDocument` vs `ReplaceDocument`.** `SetDocument` swaps the doc
  silently — used on `fileNew` and `fileOpen` where the old undo
  history is gone anyway. `ReplaceDocument` pushes a single
  `undoKindOther` entry before the swap so cmd+Z reverts. Use it
  whenever the user might want to undo the swap (e.g. version restore).

### Context menu — `internal/editor/context_menu.go`

The editor exposes a **`SetContextMenuExtender(fn)`** hook. The app
layer registers a function that returns extra menu items based on the
click target (selection text vs. word + surrounding sentence). This
keeps `internal/editor` decoupled from AI providers; the app injects AI
items without the editor knowing they exist.

When extending the menu, get `ContextMenuTarget.ReplaceWord` if you
need to swap the clicked word — the editor pre-computes the word range
and gives the app a thread-safe callback.

### AI streaming — `internal/editor/ai.go`

- **`editor.BeginAIReplace()` returns an `*AIReplace` handle.** It
  captures the original selection, pushes a single undo entry, and
  immediately empties the selection so the caret sits where streamed
  text will land.
- **`handle.Append(chunk)` / `Commit()` / `Cancel()`.** Append is safe
  to call from any goroutine — it calls `fyne.Do` internally where
  needed. Cancel reverts to the original text. Commit finalizes (no
  rollback). The whole streamed operation is one undo step.
- **`activeAI` id on the editor** prevents two streams from racing each
  other into the same selection.

### AI provider abstraction — `internal/ai`

- **`Provider` interface (Generate + Stream)** lives in `provider.go`.
  Three implementations: `AnthropicProvider`, `OpenAIProvider`,
  `MockProvider`. `Registry.NewRegistry(cfg)` picks Anthropic first,
  OpenAI second, mock last based on which keys are in `.env`.
- **OpenAI provider doesn't set `Temperature` or `MaxCompletionTokens`.**
  gpt-5 only accepts the default temperature (1.0) and counts
  reasoning tokens against the same budget as visible output. We set
  `ReasoningEffort = "minimal"` instead. Anthropic still honors
  per-command temperatures and max-tokens.
- **`templates.go` builds Request bodies from `PromptInputs`** (selection,
  sentence, document, context). To add a new built-in command:
  1. Add a `CmdXxx` constant and add it to `BuiltinCommands()`.
  2. Add system + user prompt branches in `systemPrompt`/`userPrompt`.
  3. Pick a sensible `maxTokensFor` and `temperatureFor`.
  4. Wire it into the right-click extender in `app/ai_actions.go` if
     it belongs in the context menu, or just let the palette pick it
     up automatically.

### Command palette — `internal/ui/commandpalette.go`

- **Generic — knows nothing about AI.** `app/ai_actions.go` builds the
  command list. To add a new palette entry, add a `ui.PaletteCommand`
  in `openCommandPalette()`. Disabled rows are dimmed and show a
  `· hint` suffix.
- **`escEntry`** is the reusable pattern for "modal with Esc to close":
  embeds `widget.Entry`, overrides `TypedKey` to intercept Esc and
  invoke `onEsc`. Future custom file dialogs will reuse it.

## M5 design notes

M5 adds two SQLite-backed features that ride alongside the document
without changing the editor's core text model: version snapshots
(shipped) and anchored comments (next).

### Version history — shipped (Phase A)

What's in the tree (see `internal/store/versions.go`,
`internal/app/versions.go`, `internal/ui/sidebar_versions.go`):

- **Trigger.** Snapshot fires after every successful `fileSave` /
  `fileSaveAs`. No periodic ticker — keeps the store deterministic and
  matches the user's mental model ("Save = checkpoint").
- **Dedup.** `Store.InsertVersion` skips the write when the most recent
  row for the doc already has the same `content_hash` (sha256 of the
  marshalled `doc.Document`).
- **Retention.** `Store.GCVersions(path, 50)` runs after every insert.
- **UI.** Sidebar is now an `AppTabs` (Issues / Comments / Versions).
  The Versions tab lists snapshots newest-first with a relative-time
  label + short hash. Selecting a row renders a unified diff (via
  `pmezard/go-difflib`) over `doc.WriteMarkdown` output.
- **Restore.** `editor.ReplaceDocument(d)` swaps the document and pushes
  a single `undoKindOther` entry first, so cmd+Z reverts the restore in
  one step. The doc is marked dirty afterward — the user has to save to
  persist the restored content.

### Comments — Phase B (still to build)

Anchor free-text comments to byte ranges, with a sidebar list.

- **Schema:** `comments(id, doc_path, anchor_text, range_start_hint,
  range_end_hint, body, resolved, created_at)` already exists.
- **UI:** right-click on a selection → "Add comment...", which opens
  an inline popup. The sidebar (re-use the M4 sidebar shell) lists
  open comments with author/timestamp and Resolve/Delete actions.
- **Anchoring strategy:** same approach as AI issues. Store
  `anchor_text` + the original byte offset as a hint; on doc load, try
  the offset first; if the text doesn't match, fall back to a
  substring search and update the hint.
- **Files to add:** `internal/store/comments.go`, sidebar panel in
  `internal/ui/sidebar_comments.go`, `internal/app/comments.go`.
- **Reuse:** the editor's `editor.Issue` rendering path is a near
  match for "decorate a byte range". Consider lifting it into a more
  generic `editor.Annotation` so comments + issues share the
  decoration pool without duplicating wavy-underline code.

## Why we chose what we chose

The decisions worth remembering when you're tempted to change them.

- **Custom WYSIWYG editor (not markdown-edit + preview).** User picked
  this knowing it was months-of-work risk. The whole `internal/editor`
  package exists to satisfy it. If editor work feels intractable, the
  documented fallback is markdown-edit + live preview.
- **Both Anthropic + OpenAI providers behind one interface.** Selected
  at runtime via `.env`. Default order is Anthropic > OpenAI > mock.
- **Markdown files + SQLite sidecar (not all-SQLite).** `.md` files
  stay portable; SQLite (in `~/Library/Application Support/fyne-writer`)
  holds versions/comments/prompts/settings.
- **No streaming UI affordance.** Streaming feels instant because text
  appears as it arrives; we deferred adding spinners or "thinking..."
  placeholders. Revisit in M6 polish if it feels janky on a slow
  network.
- **No font embedding yet.** Inter + JetBrains Mono are planned but
  deferred to M6; using Fyne defaults keeps the binary small during
  active development.
- **Pure-Go SQLite** (`modernc.org/sqlite`, not `mattn/go-sqlite3`) so
  cross-compilation doesn't need CGo.
- **Version snapshots on save only — no periodic ticker.** The README
  design notes mentioned a periodic snapshot ticker; we dropped it
  during implementation. Save = checkpoint matches the user's mental
  model, the store stays deterministic in tests, and we already dedup
  on `content_hash` so re-saves are free. If users start losing work
  to crashes, revisit.
- **Read-only-URL publishing deferred to v2.** Requires a backend
  service; out of scope for v1.

## Architecture

```
cmd/fyne-writer/main.go            entry point, --check-ai flag
internal/
  app/                             window, menus, AI workflow wiring
    app.go                         lifecycle + file menu (cmd+N/O/S) + sidebar
    ai_actions.go                  cmd+K palette, paraphrase, synonyms, right-click extender
    checks.go                      document-check trigger + anchor resolution
    prompts.go                     custom-prompt rendering, hotkey parser, library opener
    settings.go                    settings dialog opener + hot-apply
    versions.go                    snapshot on save + diff preview + restore wiring
    widgets.go                     shared widget helpers
  config/env.go                    .env loader, model defaults
  doc/                             document model — pure data
    model.go                       Document / Block / Inline / DocMeta
    marks.go                       Mark bitset
    position.go                    Position / Selection
    block.go                       inline-preserving InsertText/DeleteRange/ApplyMark
    blockstyle.go                  per-block-type visual style
    markdown_read.go               goldmark → Document
    markdown_write.go              Document → markdown
    frontmatter.go                 YAML front-matter parse/write
  editor/                          the custom WYSIWYG widget
    editor.go                      RichEditor (BaseWidget) + state
    layout.go                      word-wrap, visualLine, styleRun, decomposition
    renderer.go                    canvas object pool, decorations, selection rects
    commands.go                    text mutations (insert/delete/split)
    input_keys.go                  TypedRune / TypedKey / arrow nav
    input_mouse.go                 Tapped / Dragged / TappedSecondary
    shortcuts.go                   TypedShortcut, mark + block bindings
    context_menu.go                right-click menu + AI extension hook
    ai.go                          BeginAIReplace streaming handle
    issues.go                      AI-check issue state + apply/dismiss + invalidation
    undo.go                        snapshot ring (500), coalescing
  ai/                              provider abstraction + prompts
    provider.go                    Provider interface, Request / Chunk
    anthropic.go                   Anthropic backend
    openai.go                      OpenAI backend
    mock.go                        offline / no-key fallback
    registry.go                    picks active provider from config
    templates.go                   built-in commands + prompt builders
    checks.go                      document-wide grammar/clarity prompt + JSON parser
  store/
    db.go                          SQLite open + migrations
    settings.go                    KV helpers for the settings table
    prompts.go                     CRUD on the prompts table
    versions.go                    snapshot CRUD + dedup + GC
  ui/                              shared UI fragments
    theme.go                       custom fyne.Theme (light + dark) + forced variant
    commandpalette.go              cmd+K modal popup
    dialog_settings.go             Settings... dialog
    dialog_prompts.go              Prompts Library dialog + per-prompt editor
    sidebar_issues.go              AI-check sidebar widget (list + Accept/Reject)
    sidebar_versions.go            Versions sidebar widget (list + diff preview + Restore)
```

Tests live alongside the code they cover. As of M5 Phase A:

- `internal/doc/` — 22 tests (block helpers, mark + block round-trip,
  front-matter, clone, position).
- `internal/editor/` — 14 tests (selection math, mutations, undo
  coalescing).
- `internal/ai/` — JSON parser for `parseSuggestions`.
- `internal/app/` — hotkey parser + unique-anchor resolver.
- `internal/store/` — version dedup, list ordering, GC retention.

```sh
go test ./...
```

## Repository layout

- `.env.example` — placeholder for API keys.
- `.gitignore` — excludes the compiled binary, `.env`, and editor scratch.
- `CLAUDE.md` — behavioral guidelines for the AI assistant working on this
  codebase. Worth reading if you're collaborating.
