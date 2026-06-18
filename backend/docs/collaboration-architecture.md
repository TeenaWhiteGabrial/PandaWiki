# Collaboration Architecture Tasks

## Current State

- Legacy whole-string Y.Text collaboration is disabled by default.
- `/api/v1/node/collab` returns `mode=disabled` unless `collab.mode=prose_mirror`.
- ProseMirror collaboration is only eligible for non-Markdown documents.
- Node records now have `editor_content`, `editor_format`, and `editor_version` fields for structured editor state.
- Node records also have `editor_blocks` for a derived block index. It is not the source of truth yet; `editor_content` remains the editable source.
- Existing `content` remains the published/compatibility string field.

## Target Model

- Use ProseMirror JSON as the primary edit-time document model.
- Use Markdown and HTML as import/export formats, not as realtime collaboration models.
- Use Tiptap Collaboration with Yjs for rich-text structured collaboration.
- Split very large documents into block-level collaborative units.
- Persist Yjs incremental updates server-side and periodically compact them into snapshots.

## Implementation Phases

### 1. Structured Editor Model

- Convert legacy Markdown/HTML content into ProseMirror JSON on open or save.
- Store converted content in `nodes.editor_content`.
- Store a derived block index in `nodes.editor_blocks`.
- Set `nodes.editor_format=prose_mirror`.
- Increment `nodes.editor_version` when changing the structured schema.
- Keep writing `nodes.content` for existing publish, search, RAG, and share flows.

### 2. ProseMirror Collaboration

- Enable `collab.mode=prose_mirror` only after validating the editor conversion path.
- Initialize Tiptap from `editor_content` when available.
- Bind Tiptap Collaboration to the Yjs document, not to HTML or Markdown strings.
- Save by serializing editor JSON to `editor_content` and export content to `content`.
- Do not reintroduce whole-document `delete + insert` synchronization.

### 3. Block-Level Collaboration

- Introduce stable block IDs for headings, paragraphs, lists, code blocks, and tables.
- First step: keep document order and block metadata in `nodes.editor_blocks` while keeping full ProseMirror JSON in `nodes.editor_content`.
- `/api/v1/node/collab` returns `mode=block` for long documents so the admin client does not connect the full-document ProseMirror room.
- The response includes `doc_room_id`, `block_room_prefix`, and `block_window` as the future block-room scheduling contract.
- The admin client computes a block-room scheduling plan from `editor_blocks` and `block_room_prefix`; this is currently diagnostic only and does not connect block rooms yet.
- The admin client moves the block-room scheduling window based on the current Tiptap selection, throttled to avoid rebuilding block indexes on every keystroke.
- Block index items include a ProseMirror node `path`, so block-room initialization can extract the exact source node instead of relying on top-level ordering.
- Block-room content changes are merged back into a full ProseMirror JSON document before saving `editor_content`.
- Local Tiptap updates publish the active block back into its block room, while the full-document editor remains the visible editing surface.
- Later step: move block content and block order to separate persistence records once viewport-only collaboration is enabled.
- Sync only active or visible blocks.
- Keep outline, summary, and character counts asynchronous.
- Add guardrails for documents above size or block-count thresholds.

### 4. Yjs Persistence And Compaction

- Store incremental Yjs updates by room.
- Periodically compact updates into a snapshot.
- Delete compacted updates after successful snapshot creation.
- Track room size, update count, snapshot size, and connected clients.
- Reject or pause collaboration for rooms that exceed safety thresholds until compaction succeeds.
- Unload empty in-memory rooms after `COLLAB_EMPTY_ROOM_TTL_MS`.
- Enforce `COLLAB_MAX_LOADED_ROOMS` and `COLLAB_MAX_BLOCK_ROOMS_PER_USER_DOC` to prevent block-room fanout from exhausting the service.

## Guardrails

- Markdown source mode does not use realtime collaboration until a dedicated CodeMirror/Yjs path exists.
- Rich-text collaboration must use ProseMirror/Yjs structural updates.
- Long documents must not force a whole-document React state update on each keystroke.
- Any collaboration mode must have explicit backend `mode` gating.

## Acceptance Criteria

- 100 KB, 500 KB, and 1 MB documents open without freezing the editor.
- Typing latency P95 stays below 100 ms in normal documents.
- Realtime updates do not create multi-megabyte content amplification.
- Disconnect and reconnect preserve document consistency.
- Two users editing the same paragraph do not lose content.
- Editing one block in a long document does not rerender or resync the entire document.
