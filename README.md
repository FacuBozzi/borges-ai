# Fyne Writer

An AI-powered Markdown writing tool with a custom WYSIWYG editor, built
in Go on the [Fyne](https://github.com/fyne-io/fyne) toolkit.

**Resuming development?** Read this README top-to-bottom — it's the
source of truth. Sections most relevant to picking up work:

- [Current state — what works today](#current-state--what-works-today) — feature inventory.
- [Not yet built (roadmap)](#not-yet-built-roadmap) — only v2 publishing remains.
- [M6 — what shipped](#m6--what-shipped) — the polish milestone, item by item.
- [Architectural conventions](#architectural-conventions) — load-bearing patterns to follow.
- [Why we chose what we chose](#why-we-chose-what-we-chose) — context behind the decisions.
- `git log --oneline` — commit messages are detailed; each milestone is its own commit.

**Quick status (as of M6):** M0–M6 are shipped — the full v1 feature set.
The flagship features (custom WYSIWYG editor, dual-provider AI layer with
palette + context menu + custom prompts, document checks, version history,
anchored comments) plus the M6 polish pass (custom Esc-aware file dialogs,
file-browser sidebar, embedded Inter/JetBrains Mono fonts, smooth wavy
underline, status-bar word count + cmd+/ cheatsheet, first-run onboarding,
and a per-block layout cache). The only remaining v1-scope deliverable,
read-only-URL publishing, is deferred to v2 (requires a backend).

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
custom prompts, the Settings dialog, per-save version history, anchored
comments, and the M6 polish pass (M0–M6 in the internal milestone plan)
are all shipped. The read-only-URL publishing piece is the only major v1
deliverable left, and it's deferred to v2 (requires a backend).

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
| Custom Esc-aware open/save dialogs | ✓ | `internal/ui/dialog_file.go`; remembers last-used dir |
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
| **Anchored comments** | ✓ | Right-click → Add comment…; yellow background highlight; Comments sidebar with Jump / Resolve / Delete; survives doc edits via offset hint + substring fallback |

### Storage

| Feature | Status | Notes |
|---|---|---|
| SQLite store with WAL + foreign keys | ✓ | At `~/Library/Application Support/fyne-writer/fyne-writer.db` on macOS |
| Schema for settings / versions / comments / prompts | ✓ | All shipped. |
| **Version snapshots on save** | ✓ | Versions tab in sidebar; unified diff preview; Restore is one undo step. Dedup on identical content hash. Capped at 50 per doc. |

### UI shell & polish (M6)

| Feature | Status | Notes |
|---|---|---|
| **File-browser sidebar tab** | ✓ | 4th sidebar tab; single-click opens a `.md`; tracks the active doc's dir; warns on unsaved changes |
| **Embedded fonts** | ✓ | Inter (proportional) + JetBrains Mono (monospace), SIL OFL, `//go:embed` in `internal/ui/fonts.go` |
| **Status bar word count + dirty pip** | ✓ | Live word count over block plain text; `●` when unsaved |
| **Keyboard cheatsheet** | ✓ | cmd+/ ; filterable modal (`Help` menu) |
| **First-run onboarding** | ✓ | Welcome modal on first launch; gated by the `onboarding_done` setting |
| **Per-block layout cache** | ✓ | Memoizes wrap results so only edited blocks re-wrap (`internal/editor/layout_cache.go`) |

## Not yet built (roadmap)

- **Deferred to v2 — read-only URL publishing.** Requires a backend
  service (out of scope for v1).

## Known limitations

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
- **First Synonyms lookup is still a popup.** The right-click submenu
  shows cached synonyms inline, but the *first* lookup of a word opens
  the async popup (the AI call can't populate a submenu after the parent
  menu has closed). We chose a cache-only submenu over hover-prefetch to
  avoid firing speculative paid API calls as the cursor moves.
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
- **Issues vs Comments — parallel state, shared invalidation.** Both
  `issues []Issue` and `comments []Comment` live on the editor with
  near-identical state-management code. The shared piece is
  `anchorStillMatches()` — both validators call it from `invalidate()`
  after every mutation. The visual decoration is separate: issues use
  the red zigzag `issueDeco` pool; comments use the yellow `commentBGs`
  rectangle pool. We did not unify the structs because the visual
  shapes differ (zigzag stroke vs background fill), so the would-be
  shared rendering code is empty.

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

## M5 design notes — what shipped

M5 added two SQLite-backed features that ride alongside the document
without changing the editor's core text model: version snapshots and
anchored comments. Both are shipped.

### Version history (Phase A)

What's in the tree (see `internal/store/versions.go`,
`internal/app/versions.go`, `internal/ui/sidebar_versions.go`):

- **Trigger.** Snapshot fires after every successful `fileSave` /
  `fileSaveAs`. No periodic ticker — keeps the store deterministic and
  matches the user's mental model ("Save = checkpoint").
- **Dedup.** `Store.InsertVersion` skips the write when the most recent
  row for the doc already has the same `content_hash` (sha256 of the
  marshalled `doc.Document`).
- **Retention.** `Store.GCVersions(path, 50)` runs after every insert.
- **UI.** Sidebar is an `AppTabs` (Issues / Comments / Versions). The
  Versions tab lists snapshots newest-first with a relative-time label
  + short hash. Selecting a row renders a unified diff (via
  `pmezard/go-difflib`) over `doc.WriteMarkdown` output.
- **Restore.** `editor.ReplaceDocument(d)` swaps the document and pushes
  a single `undoKindOther` entry first, so cmd+Z reverts the restore in
  one step. The doc is marked dirty afterward — the user has to save to
  persist the restored content.

### Comments (Phase B)

What's in the tree (see `internal/store/comments.go`,
`internal/app/comments.go`, `internal/ui/sidebar_comments.go`,
`internal/editor/comments.go`):

- **Trigger.** Right-click → "Add comment…" on a single-block
  selection. Opens a Fyne dialog with a multi-line textarea. Save
  persists + decorates.
- **Anchor model.** `comments(block_index, range_start_hint,
  range_end_hint, anchor_text)`. On doc load, resolver tries the hint
  first; if the in-block range no longer matches `anchor_text`, falls
  back to `findUniqueAnchor` (same uniqueness rule as issues). If the
  fallback succeeds, the new hint is persisted.
- **Schema migration.** `block_index` was added in Phase B with an
  in-code `ensureColumn` check (SQLite lacks `ADD COLUMN IF NOT
  EXISTS`) so dev DBs from earlier milestones don't trip.
- **Decoration.** Translucent yellow background rectangle (`commentBGs`
  pool, sized like `selRects`). Drawn behind selection so a selection
  + comment overlap still shows the selection color. The wavy issue
  underline is unchanged — comments and issues share state-management
  code (`anchorStillMatches` helper, the same invalidation hook) but
  paint independently.
- **Sidebar.** Lists open comments with Jump / Resolve / Delete.
  Comments whose anchor no longer resolves are kept in the list as
  "orphaned" (Jump disabled).

## M6 — what shipped

M6 was polish + onboarding: no new flagship features, just the papercut
backlog from M0–M5 plus first-run guidance. Each item is its own commit
(`git log`, prefixed `M6 #N`).

1. **Custom file dialogs** (`internal/ui/dialog_file.go`, `modal.go`,
   `dirlist.go`). Fyne's built-in `dialog.FileDialog` focuses an internal
   `widget.Entry` that swallows Esc; our `escEntry`-based open/save modals
   close on Esc and remember the last-used directory (`last_open_dir`
   setting). The `showModal` helper and the `dirList` browser were extracted
   here and reused by later items.
2. **File-browser sidebar** (`internal/ui/sidebar_files.go`). A 4th sidebar
   tab over `dirList`; single-click opens a `.md` through the same `openFile`
   path the menu uses (so versions + comments reload), warning first on
   unsaved changes.
3. **Embedded fonts** (`internal/ui/fonts.go`, `assets/fonts/`). Inter +
   JetBrains Mono static TTFs (SIL OFL) embedded via `//go:embed` and served
   from `FyneWriterTheme.Font` — Inter for proportional text, JetBrains Mono
   for monospace. Symbol font still defers to the Fyne default.
4. **Smooth wavy underline** (`renderer.go` `syncIssueUnderlines`). The
   AI-check underline is now a sampled sine polyline (period 6px, amplitude
   1.5px) instead of a straight-segment zigzag.
5. **Status bar + cheatsheet.** `refreshStatus` shows a live word count (over
   block plain text, so markdown syntax doesn't inflate it) and a `●` dirty
   pip; cmd+/ opens a filterable keyboard cheatsheet
   (`internal/ui/dialog_shortcuts.go`, `Help` menu).
6. **First-run onboarding** (`internal/ui/dialog_onboarding.go`). A welcome
   modal shows once on first launch, gated by the `onboarding_done` setting;
   `Run()` splits `Show()`+`Run()` so it overlays after the window appears.
7. **Per-block layout cache** (`internal/editor/layout_cache.go`). `layout()`
   re-wrapped every block on every Refresh; `layoutCached` memoizes
   `wrapBlock` output keyed by (plain text, font size, indent, bold, mono,
   width). Keying on the plain-text *value* means undo/redo/restore clones
   hit the cache with no pointer bookkeeping; the outer loop still re-stamps
   absolute geometry each call. A width change rebuilds the map;
   `InvalidateLayoutCache` busts it on a theme/font swap.
   `layout_cache_test.go` asserts the cached output is byte-identical to the
   pure `layout()`.

The small-polish batch also shipped: a comments "show resolved" toggle, a
bottom-bar streaming spinner (`widget.Activity`) during AI replacement, and
a cache-only Synonyms inline submenu (right-click a word → cached synonyms
appear as clickable children after the first lookup, no speculative calls).

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
- **Pure-Go SQLite** (`modernc.org/sqlite`, not `mattn/go-sqlite3`) so
  cross-compilation doesn't need CGo.
- **Version snapshots on save only — no periodic ticker.** The README
  design notes mentioned a periodic snapshot ticker; we dropped it
  during implementation. Save = checkpoint matches the user's mental
  model, the store stays deterministic in tests, and we already dedup
  on `content_hash` so re-saves are free. If users start losing work
  to crashes, revisit.
- **Comments use yellow background, not a wavy underline.** The
  original M5 design notes hinted at sharing the AI-check wavy-underline
  pool. We diverged: yellow background reads as "annotated" rather than
  "wrong", and stays visually distinct from issues. The state-management
  code is still shared (via `anchorStillMatches`); only the painter
  differs. See [Architectural conventions → Issues vs Comments]
  (#architectural-conventions).
- **Idempotent `ensureColumn` helper for additive schema migrations.**
  SQLite has no `ADD COLUMN IF NOT EXISTS`, so a `PRAGMA table_info`
  check guards each `ALTER TABLE` in `store/db.go`. Use it for any
  future column-add that has to survive on dev DBs created before the
  change.
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
    comments.go                    Add comment dialog + anchor resolver + jump/resolve/delete
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
    layout_cache.go                per-block wrap memoization (M6 perf)
    renderer.go                    canvas object pool, decorations, selection rects
    commands.go                    text mutations (insert/delete/split)
    input_keys.go                  TypedRune / TypedKey / arrow nav
    input_mouse.go                 Tapped / Dragged / TappedSecondary
    shortcuts.go                   TypedShortcut, mark + block bindings
    context_menu.go                right-click menu + AI extension hook
    ai.go                          BeginAIReplace streaming handle
    issues.go                      AI-check issue state + apply/dismiss + invalidation
    comments.go                    Comment state + add/remove + invalidation (shared anchor helper)
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
    db.go                          SQLite open + migrations + ensureColumn helper
    settings.go                    KV helpers for the settings table
    prompts.go                     CRUD on the prompts table
    versions.go                    snapshot CRUD + dedup + GC
    comments.go                    comment CRUD + resolved filter + anchor-hint update
  ui/                              shared UI fragments
    theme.go                       custom fyne.Theme (light + dark) + forced variant
    fonts.go                       embedded Inter + JetBrains Mono faces
    assets/fonts/                  vendored .ttf + OFL license files
    modal.go                       showModal overlay helper (shared by dialogs)
    dirlist.go                     reusable os.ReadDir directory browser
    commandpalette.go              cmd+K modal popup
    dialog_file.go                 custom Esc-aware open/save pickers
    dialog_settings.go             Settings... dialog
    dialog_prompts.go              Prompts Library dialog + per-prompt editor
    dialog_shortcuts.go            cmd+/ keyboard cheatsheet
    dialog_onboarding.go           first-run welcome modal
    sidebar_issues.go              AI-check sidebar widget (list + Accept/Reject)
    sidebar_versions.go            Versions sidebar widget (list + diff preview + Restore)
    sidebar_comments.go            Comments sidebar widget (list + Jump/Resolve/Delete)
    sidebar_files.go               File-browser sidebar widget (dirList tab)
```

Tests live alongside the code they cover. As of M6:

- `internal/doc/` — block helpers, mark + block round-trip,
  front-matter, clone, position.
- `internal/editor/` — selection math, mutations, undo coalescing, and
  the layout cache (asserts `layoutCached` matches the pure `layout()`).
- `internal/ai/` — JSON parser for `parseSuggestions`.
- `internal/app/` — hotkey parser + unique-anchor resolver.
- `internal/store/` — version dedup, list ordering, GC retention,
  comment CRUD + resolved filter.

```sh
go test ./...
```

## Repository layout

- `.env.example` — placeholder for API keys.
- `.gitignore` — excludes the compiled binary, `.env`, and editor scratch.
- `CLAUDE.md` — behavioral guidelines for the AI assistant working on this
  codebase. Worth reading if you're collaborating.
