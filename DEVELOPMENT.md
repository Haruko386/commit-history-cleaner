# Git History Cleaner — Development Guide

> **Document purpose**  
> This file is the single source of truth for collaboration between the project owner and the coding Agent.  
> The project owner is responsible for the Go backend. The Agent is responsible for building and polishing the frontend, coordinating frontend–backend contracts, tracking feature completion, and reporting progress.

---

## 1. Project Overview

### 1.1 Product positioning

A desktop Git history analyzer and cleanup assistant built with:

- **Frontend:** Vue 3 + TypeScript
- **Desktop framework:** Wails
- **Backend:** Go
- **Git integration:** system Git CLI first; additional libraries may be evaluated later

The application should allow a user to select a local Git repository, inspect commit history in a GitHub-like visual timeline, analyze commit/file/object sizes, identify large historical objects, select cleanup targets, and generate safe Git cleanup commands.

### 1.2 Core product goal

The product should make the following workflow easy to understand:

1. Select a local Git repository.
2. Scan commit history.
3. Display commits visually.
4. Show how much data each commit introduced.
5. Drill down into large files / blobs.
6. Identify files deleted from HEAD but still retained in Git history.
7. Select commits/files/objects to clean.
8. Preview the impact.
9. Generate Git commands.
10. Only execute destructive actions after explicit user confirmation and after backup guidance.

---

## 2. Collaboration Roles

## 2.1 Project Owner — Backend Owner

The project owner is responsible for:

- Go backend architecture.
- Wails backend bindings.
- Git command execution.
- Repository scanning.
- Commit/object/file analysis.
- Cleanup planning logic.
- Safety checks.
- Final decision on backend API design.
- Final decision on destructive Git operations.

The backend is considered **owner-controlled code**.

---

## 2.2 Agent — Frontend Engineer + Project Coordinator

The Agent's primary role is:

> **Build, improve, refactor, and maintain the frontend while helping the owner identify backend problems without writing backend code unless explicitly authorized.**

The Agent may:

- Create and modify frontend Vue / TypeScript / CSS files.
- Improve UI/UX.
- Build reusable frontend components.
- Implement routing, state management, tables, timelines, dialogs, charts, and interaction logic.
- Connect frontend components to existing Wails bindings.
- Read backend code to understand available APIs.
- Inspect backend behavior when necessary for frontend integration.
- Detect backend API problems.
- Explain backend bugs or missing capabilities.
- Propose backend corrections in natural language.
- Propose request/response field changes in tables or prose.
- Point out the exact backend file/function/module that likely needs adjustment.
- Maintain this development document.
- Update feature completion checkboxes.
- Update completion percentages.
- Maintain issue, dependency, and integration records.

---

## 2.3 Agent Backend Restrictions

### HARD RULE

The Agent **MUST NOT write, modify, patch, replace, or generate Go backend implementation code unless the project owner explicitly authorizes it.**

This applies even if:

- the backend implementation is obviously incomplete;
- frontend work is blocked;
- the Agent knows the exact fix;
- the change appears trivial;
- the Agent believes the fix is safe.

Without explicit authorization, the Agent must only provide a backend correction report.

### Minimal-change rule for authorized backend work

Even when backend changes are explicitly authorized, the Agent must make the smallest possible patch and preserve the owner's naming, structure, control flow, and reasoning style.

- Change only the lines required by the owner's request.
- Do not rewrite or broadly refactor an existing backend function when an in-place fix is sufficient.
- Do not extract logic into a separate method unless it is reused roughly four or five times, or the original function is clearly too long to remain understandable.
- Tests may be added, but adding tests does not authorize restructuring the implementation.
- If a fix would require a broad redesign, stop and ask the owner before changing it.

### Allowed backend assistance without authorization

The Agent may provide:

- problem description;
- suspected root cause;
- affected backend module;
- expected behavior;
- required API fields;
- suggested function responsibilities;
- data-flow explanation;
- compatibility concerns;
- validation requirements;
- edge cases;
- test cases the owner should consider.

### Forbidden backend actions without authorization

The Agent must not:

- edit `.go` files;
- create new backend `.go` files;
- apply patches to backend code;
- rewrite backend functions;
- generate copy-paste Go implementations;
- silently change Wails backend exports;
- implement Git command execution logic;
- implement destructive repository operations.

### Authorization wording

Backend coding is permitted only when the owner clearly says something equivalent to:

- “你可以改后端”
- “这个 Go 文件你直接帮我写”
- “允许你实现这个后端接口”
- “现在可以修改 backend”
- “把这个函数写出来”

Authorization applies only to the requested scope unless the owner explicitly grants broader permission.

---

## 3. Agent Working Principles

The Agent should follow these principles in order:

1. **Do not break existing behavior.**
2. **Frontend first.**
3. **Respect backend ownership.**
4. **Prefer incremental features over large rewrites.**
5. **Keep frontend components reusable.**
6. **Keep destructive operations visually separated from read-only operations.**
7. **Never hide dangerous Git behavior behind a single casual button.**
8. **Always provide a preview before destructive operations.**
9. **Track every feature in this document.**
10. **Report progress after each completed feature.**

---

# 4. Development Workflow

Every feature should follow this lifecycle:

```text
Feature proposed
      ↓
Requirement clarified
      ↓
Frontend UI/API contract designed
      ↓
Backend dependency checked
      ↓
Frontend implemented
      ↓
Integration tested
      ↓
Feature reviewed
      ↓
Checklist updated
      ↓
Progress percentage recalculated
      ↓
Commit
```

---

## 4.1 Feature Start Procedure

Before implementing a feature, the Agent should record:

| Field | Description |
|---|---|
| Feature | Feature name |
| Goal | What the user should be able to do |
| Frontend scope | Components/pages/state involved |
| Backend dependency | Existing API / missing API / unknown |
| Risk | Low / Medium / High |
| Status | Todo / In Progress / Blocked / Done |

If the backend API is missing, the Agent must **not invent backend behavior**.

Instead, add a Backend Request entry.

---

## 4.2 Feature Completion Procedure

A feature may be marked complete only when:

- [ ] UI is implemented.
- [ ] Main interaction works.
- [ ] Empty state is handled.
- [ ] Loading state is handled.
- [ ] Error state is handled.
- [ ] Wails/API integration is connected or clearly mocked when authorized.
- [ ] No known regression is introduced.
- [ ] Relevant feature checklist is updated.
- [ ] Progress percentage is updated.
- [ ] Completion report is written.

---

## 4.3 Completion Report Format

After completing one feature, the Agent should report:

```text
Feature completed: <feature name>

Frontend:
- <what was implemented>
- <important interaction changes>

Backend:
- No backend changes
or
- Backend adjustment required: <short explanation>

Files changed:
- <frontend file>
- <frontend file>

Testing:
- <what was verified>

Project progress:
XX% → YY%

Next recommended feature:
<feature>
```

---

# 5. Progress Tracking Rules

## 5.1 Overall Progress

Overall progress is calculated from weighted milestones.

| Milestone | Weight |
|---|---:|
| M0 Project Shell | 5% |
| M1 Repository Selection | 10% |
| M2 Commit History | 20% |
| M3 Commit Size Analysis UI | 15% |
| M4 Historical Object Explorer | 15% |
| M5 Cleanup Selection | 10% |
| M6 Rewrite Preview | 10% |
| M7 Command Generator | 10% |
| M8 Polish / Release | 5% |
| **Total** | **100%** |

A milestone's contribution is:

```text
milestone contribution = milestone weight × completed feature ratio
```

Overall progress:

```text
overall progress = sum(all milestone contributions)
```

The Agent should update the percentage after every meaningful completed feature.

---

# 6. Product Roadmap

# M0 — Project Shell — 5%

### Goal

Create a stable Wails + Vue application shell.

- [ ] Wails application starts correctly.
- [x] Vue frontend loads correctly.
- [x] TypeScript configured.
- [x] Global layout created.
- [x] Sidebar/navigation structure created.
- [x] Global design tokens created.
- [ ] Reusable button/input/dialog styles created.
- [x] Basic error boundary / error presentation prepared.

**Milestone progress:** `6 / 8`  
**Completion:** `75% of M0`  
**Overall contribution:** `3.75%`

---

# M1 — Repository Selection — 10%

### Goal

Allow the user to select and validate a local Git repository.

- [x] Welcome page.
- [x] “Open Repository” action.
- [ ] Recent repository list UI.
- [x] Repository path display.
- [x] Invalid repository state.
- [x] Repository metadata summary.
- [x] Loading state while opening repository.
- [ ] Close / switch repository action.

Suggested repository summary:

| Field | Example |
|---|---|
| Repository | Haruko386.github.io |
| Branch | master |
| HEAD | `1ce1dd6` |
| Commits | 326 |
| Working Tree | Clean |
| `.git` Size | 1.82 GB |

**Milestone progress:** `6 / 8`
**Completion:** `75% of M1`
**Overall contribution:** `7.50%`

---

# M2 — Commit History — 20%

### Goal

Display Git commit history with a GitHub-like experience.

- [ ] Commit timeline page.
- [ ] Commit grouping by date.
- [ ] Commit hash.
- [ ] Commit message.
- [ ] Author.
- [ ] Timestamp.
- [ ] Parent relationship support.
- [ ] Branch indicator.
- [ ] Tag indicator.
- [ ] Commit status / selected state.
- [ ] Commit multi-select.
- [ ] Search by commit message.
- [ ] Search by hash.
- [ ] Author filter.
- [ ] Date filter.
- [ ] Virtualized rendering for large repositories.
- [ ] Commit detail drawer/panel.
- [ ] Empty state.
- [ ] Error state.
- [ ] Refresh history action.

**Milestone progress:** `0 / 20`  
**Completion:** `0% of M2`  
**Overall contribution:** `0.00%`

---

# M3 — Commit Size Analysis UI — 15%

### Goal

Make repository growth visually understandable.

Each commit should preferably display:

- Introduced size
- Added files
- Deleted files
- Modified files

Features:

- [ ] Introduced size displayed per commit.
- [ ] Human-readable size formatting.
- [ ] Size severity indicator.
- [ ] Expand commit file changes.
- [ ] File size column.
- [ ] File status indicator.
- [ ] Sort files by size.
- [ ] Highlight unusually large additions.
- [ ] Commit size summary panel.
- [ ] Snapshot size field support.
- [ ] Tooltip explaining “Introduced Size”.
- [ ] Tooltip explaining “Snapshot Size”.

Recommended terminology:

| Term | Meaning |
|---|---|
| Introduced Size | New blob/file data introduced relative to parent |
| Snapshot Size | Logical size of the repository tree at the commit |
| Reclaimable Size | Estimated storage removable by rewriting history |

**Milestone progress:** `0 / 12`  
**Completion:** `0% of M3`  
**Overall contribution:** `0.00%`

---

# M4 — Historical Object Explorer — 15%

### Goal

Find the real causes of repository bloat.

- [ ] Large Objects page.
- [ ] Historical-only files view.
- [ ] Current HEAD files view.
- [ ] Blob/object size display.
- [ ] Object path display.
- [ ] First-seen commit.
- [ ] Last-seen commit.
- [ ] Current HEAD reference status.
- [ ] Sort by size.
- [ ] Filter by extension.
- [ ] Filter historical-only objects.
- [ ] File history drawer.
- [ ] Object occurrence list.
- [ ] Storage overview cards.
- [ ] Repository storage map/summary.

Suggested overview:

```text
.git size             1.82 GB
Working tree          126 MB
Historical-only       1.31 GB
Largest blob          381 MB
```

**Milestone progress:** `0 / 15`  
**Completion:** `0% of M4`  
**Overall contribution:** `0.00%`

---

# M5 — Cleanup Selection — 10%

### Goal

Allow the user to safely select cleanup targets.

- [ ] Select commit.
- [ ] Select file.
- [ ] Select historical blob/object.
- [ ] Multi-select.
- [ ] Selection summary.
- [ ] Clear selection.
- [ ] Duplicate selection handling.
- [ ] Unsafe operation warning.
- [ ] Selection conflict warning.
- [ ] Cleanup target classification.

Possible target types:

| Type | Example |
|---|---|
| Path | `static/demo.gif` |
| Commit | `f6ff93b` |
| Object | Git blob SHA |
| Pattern | `*.mp4` |

**Milestone progress:** `0 / 10`  
**Completion:** `0% of M5`  
**Overall contribution:** `0.00%`

---

# M6 — Rewrite Preview — 10%

### Goal

Show the consequences before history rewriting.

- [ ] Preview page/dialog.
- [ ] Selected targets summary.
- [ ] Affected commit count.
- [ ] Affected branch list.
- [ ] Affected tag list.
- [ ] Estimated reclaimable storage.
- [ ] Commit SHA rewrite warning.
- [ ] Remote push warning.
- [ ] Collaborator impact warning.
- [ ] Backup recommendation.
- [ ] Preview confirmation checkbox.
- [ ] Cancel action.

Suggested preview:

```text
Rewrite Plan

Targets:
- static/demo.gif
- assets/result.mp4

Affected commits: 42
Branches affected: master
Tags affected: v1.0, v1.1

Estimated reclaimable space: 565 MB

WARNING:
History rewriting changes commit IDs.
```

**Milestone progress:** `0 / 12`  
**Completion:** `0% of M6`  
**Overall contribution:** `0.00%`

---

# M7 — Command Generator — 10%

### Goal

Generate safe, understandable Git commands.

- [ ] Command preview area.
- [ ] Copy command.
- [ ] Copy all commands.
- [ ] Syntax highlighting.
- [ ] Explain each generated command.
- [ ] Backup command section.
- [ ] Cleanup command section.
- [ ] Verification command section.
- [ ] Remote update command section.
- [ ] `--force-with-lease` warning.
- [ ] Command generation error state.
- [ ] Export commands to text file.

The UI should visually separate:

```text
1. Backup
2. Rewrite
3. Verify
4. Push
```

**Milestone progress:** `0 / 12`  
**Completion:** `0% of M7`  
**Overall contribution:** `0.00%`

---

# M8 — Polish / Release — 5%

### Goal

Prepare a stable first public release.

- [ ] Responsive desktop layout.
- [ ] Light theme.
- [ ] Dark theme.
- [ ] Keyboard navigation.
- [ ] Large repository performance review.
- [ ] Error messages polished.
- [ ] Confirmation dialogs reviewed.
- [ ] Windows packaging tested.
- [ ] Application icon.
- [ ] About page.
- [ ] Version display.
- [ ] README screenshots.
- [ ] First release notes.

**Milestone progress:** `0 / 13`  
**Completion:** `0% of M8`  
**Overall contribution:** `0.00%`

---

# 7. Current Project Progress

> Agent must update this section after completing features.

| Milestone | Weight | Completed | Progress | Contribution |
|---|---:|---:|---:|---:|
| M0 Project Shell | 5% | 6 / 8 | 75% | 3.75% |
| M1 Repository Selection | 10% | 6 / 8 | 75% | 7.50% |
| M2 Commit History | 20% | 0 / 20 | 0% | 0.00% |
| M3 Commit Size Analysis UI | 15% | 0 / 12 | 0% | 0.00% |
| M4 Historical Object Explorer | 15% | 0 / 15 | 0% | 0.00% |
| M5 Cleanup Selection | 10% | 0 / 10 | 0% | 0.00% |
| M6 Rewrite Preview | 10% | 0 / 12 | 0% | 0.00% |
| M7 Command Generator | 10% | 0 / 12 | 0% | 0.00% |
| M8 Polish / Release | 5% | 0 / 13 | 0% | 0.00% |
| **TOTAL** | **100%** | — | — | **11.25%** |

## Current overall completion

# `11.25%`

---

# 8. Backend API Coordination

The Agent must treat backend interfaces as contracts.

Do not guess backend implementation details.

## 8.1 API Contract Record

| ID | Feature | Frontend Needs | Backend Status | Owner Decision | Status |
|---|---|---|---|---|---|
| API-001 | Open repository | repository metadata | `POST /api/v1/repositories/open` implemented, tested, and connected | HTTP first | Integrated |
| API-002 | Commit history | paginated commit list | Contract drafted; not implemented | HTTP first | Contract Drafted |
| API-003 | Commit files | changed files + sizes | Contract drafted; not implemented | HTTP first | Contract Drafted |
| API-004 | Object analysis | historical object list | Contract drafted; not implemented | HTTP first | Contract Drafted |
| API-005 | Cleanup preview | affected refs + estimate | Contract drafted; not implemented | HTTP first | Contract Drafted |
| API-006 | Command generation | generated command plan | Contract drafted; not implemented | HTTP first | Contract Drafted |
| API-007 | Current repository | restore and close the active repository | `GET` and `DELETE /api/v1/repositories/current` implemented and tested; frontend not connected | HTTP first | Ready for Integration |

---

## 8.2 Backend Request Template

### Backend Request `BE-001`

**Related feature:**  
Repository selection

**Frontend requirement:**  
Open a native directory picker, validate that the selected directory is a Git repository, and return repository metadata for the workspace header and overview.

**Current backend behavior:**  
`POST /api/v1/repositories/open` validates and normalizes a local path and returns repository metadata using the shared response envelope.

**Problem:**  
Resolved for the HTTP frontend phase. The frontend still needs to call the endpoint and render its loading, success, and error states.

**Suggested backend responsibility:**  
Implemented for repository validation, Git metadata, normalized error responses, empty repositories, and detached HEAD. Bare repositories and linked worktrees remain future edge cases. A later Wails adapter should reuse the same business service and response model.

**Suggested response fields:**

| Field | Type | Required | Meaning |
|---|---|---:|---|
| `path` | `string` | Yes | Absolute local repository path |
| `name` | `string` | Yes | Display name derived from the repository directory |
| `branch` | `string \| null` | Yes | Current branch, or null for detached HEAD |
| `head` | `string` | Yes | Full HEAD commit SHA |
| `commitCount` | `number` | Yes | Reachable commit count for the selected scope |
| `workingTreeStatus` | `"clean" \| "dirty"` | Yes | Simplified working-tree state |
| `gitDirectoryBytes` | `number` | Yes | Size of the Git directory in bytes |

**Edge cases:**

- User cancels the native directory picker.
- Selected path does not exist or is not readable.
- Selected path is not a Git repository.
- Repository uses a worktree or has a separate Git directory.
- Repository is in detached HEAD state or has no commits yet.
- Git CLI is unavailable.

**Frontend blocked:** No

**Backend code written by Agent:** **YES — explicitly authorized by the owner on 2026-10-07**

---

When frontend work requires backend changes, the Agent must create an entry like this:

### Backend Request `BE-XXX`

**Related feature:**  
`<feature>`

**Frontend requirement:**  
Describe what the frontend needs.

**Current backend behavior:**  
Describe what currently exists.

**Problem:**  
Describe the mismatch.

**Suggested backend responsibility:**  
Describe behavior in prose only.

**Suggested response fields:**

| Field | Type | Required | Meaning |
|---|---|---:|---|
| `...` | `...` | Yes | ... |

**Edge cases:**

- ...
- ...

**Frontend blocked:** Yes / No

**Backend code written by Agent:** **NO**

---

# 9. Backend Issue Log

| ID | Date | Module | Problem | Severity | Frontend Blocked | Status |
|---|---|---|---|---|---|---|
| BE-001 | 2026-10-06 | Repository selection | No HTTP route for repository selection and metadata | High | No | Resolved |
| BE-002 | 2026-10-07 | Scan / repository exit | Replace the temporary always-success exit behavior when scan cancellation is implemented | Medium | No | Open |
| BE-003 | 2026-10-07 | Service boundary | Remove HTTP status codes from `RepositoriesSvr` before the scan/Wails service boundary is finalized | Medium | No | Open |

Status options:

- Open
- Owner Reviewing
- Owner Implementing
- Ready for Retest
- Resolved
- Won't Fix

---

# 10. Frontend Feature Log

| ID | Feature | Status | Started | Completed | Notes |
|---|---|---|---|---|---|
| FE-001 | App Shell | In Progress | 2026-10-06 | — | Vue/TypeScript shell, navigation, tokens, buttons, and notices implemented; Wails startup remains |
| FE-002 | Repository Picker | In Progress | 2026-10-06 | — | Path input, loading, errors, and repository summary are integrated; recent/current state remains |
| FE-003 | Commit Timeline | Todo | — | — | — |
| FE-004 | Commit Detail | Todo | — | — | — |
| FE-005 | Large Object Explorer | Todo | — | — | — |
| FE-006 | Cleanup Selection | Todo | — | — | — |
| FE-007 | Rewrite Preview | Todo | — | — | — |
| FE-008 | Command Preview | Todo | — | — | — |

Status options:

- Todo
- In Progress
- Blocked
- Review
- Done

---

# 11. Decision Log

Important product and architecture decisions should be recorded here.

| ID | Date | Decision | Reason | Status |
|---|---|---|---|---|
| DEC-001 | Initial | Vue + Wails + Go | Native desktop UX with Go backend | Accepted |
| DEC-002 | Initial | Owner controls backend | Clear ownership boundary | Accepted |
| DEC-003 | Initial | Agent owns frontend | Faster UI iteration | Accepted |
| DEC-004 | Initial | System Git preferred initially | Maximum compatibility with existing Git workflow | Accepted |
| DEC-005 | Initial | Preview before destructive operations | Reduce risk of repository damage | Accepted |
| DEC-006 | 2026-10-06 | Build the web frontend against HTTP before Wails integration | Finish and test browser UI first while keeping a replaceable service adapter | Accepted |
| DEC-007 | 2026-10-07 | Authorized backend edits must remain minimal and preserve the owner's code structure | Keep backend changes understandable and under owner control | Accepted |

---

# 12. Change Log

The Agent should append a short entry after meaningful development sessions.

## 2026-10-07

### Added

- Implemented `POST /api/v1/repositories/open` with normalized repository metadata and stable error responses.
- Added repository API coverage for empty repositories, detached HEAD, malformed requests, missing paths, and invalid repositories.
- Added lifecycle coverage for opening, reading, and closing the current repository, including empty repositories and repeated close requests.
- Connected the frontend to health, repository-open, and GitHub-connection APIs with loading, success, and error states.

### Changed

- Normalized repository paths and separated stable repository IDs from per-request IDs.
- Updated repository metadata to use nullable `branch` and `head` fields and the documented camelCase JSON contract.
- Moved API-001 and BE-001 to ready/resolved; FE-002 is no longer backend-blocked.
- Added an implementation checklist to `internal/FRONTEND_API.md`; `/health` and `/repositories/open` are complete for the current Web MVP scope.
- Added explicit GitHub `401/403` responses and documented the Bearer-token contract.
- Marked the current-repository `GET` and `DELETE` API contracts ready for frontend integration.

### Fixed

- Corrected HTTP status mapping, empty-repository behavior, detached HEAD output, the standard `.git` size path, and the router test dependency setup.
- Rolled back the broad repository-service refactor and restored the owner's original control flow with only local fixes.
- Completed and tested the GitHub connection handler using a Bearer token in the Authorization header.

### Progress

```text
Before: 5%
After:  11.25%
Change: +6.25% — repository opening is integrated; recent/current state remains.
```

---

## 2026-10-06

### Added

- Vue 3 + TypeScript + Vite frontend workspace.
- Responsive application shell, sidebar navigation, welcome page, workflow overview, and recent-repository empty state.
- Reusable primary/secondary button styling, global design tokens, and backend-unavailable notice presentation.
- HTTP-first frontend API contract at `internal/FRONTEND_API.md`, including repository, task, history, object, cleanup preview, and command-generation endpoints.
- GitHub official API and history-rewrite references appended to the frontend API contract.
- Request ID middleware, reusable success/error response DTOs, and automated health endpoint tests.

### Changed

- Updated M0 and M1 milestone tracking to reflect the implemented frontend work.
- Simplified the welcome page to follow GitHub Primer styling: repository header, underline tabs, bordered boxes, standard colors, and compact typography.
- Replaced the starter branding with an Octocat-inspired GitHub variant logo and matching favicon.
- Limited `/api/v1/health` to local service and Git CLI checks; GitHub connectivity remains a separate future endpoint.
- Bound the HTTP development server to `127.0.0.1` and added a Vite `/api` proxy.
- Centralized Go tests under the root `test/` package and converted them to exercise public behavior only.

### Fixed

- Removed the default Vite demonstration UI and assets.
- Normalized health JSON responses, Git version output, request IDs, error codes, and versioned routing.

### Backend requests

- BE-001 requests the HTTP-first repository picker contract; Wails will later reuse the same service and response model.

### Progress

```text
Before: 0%
After:  5%
Change: +5%
```

---

# 13. Session Report

At the end of each development session, the Agent should produce this summary.

## Session Summary

**Completed today**

- [x] Initialize Vue 3 + TypeScript frontend.
- [x] Implement the repository welcome page and application shell.
- [x] Verify TypeScript and production build.

**In progress**

- [ ] Complete the web UI against the HTTP service contract.
- [ ] Add reusable input and dialog components.

**Blocked**

- [ ] Repository picker integration.

**Backend requests**

- BE-001 HTTP repository selection, validation, and repository metadata.

**Known issues**

- Repository, history, object analysis, and cleanup HTTP routes are not implemented yet.
- Screenshot-based browser QA was unavailable in this environment; production compilation passed.

**Overall progress**

```text
5%
```

**Recommended next step**

```text
Implement the P0 HTTP endpoints in internal/FRONTEND_API.md; then connect the repository picker and loading/error states.
```

---

# 14. Git / Commit Workflow

Recommended branch strategy:

```text
main
  ↑
dev
  ↑
feat/<feature-name>
```

Recommended commit prefixes:

| Prefix | Meaning |
|---|---|
| `feat:` | New feature |
| `fix:` | Bug fix |
| `refactor:` | Structural change |
| `style:` | Visual-only frontend change |
| `docs:` | Documentation |
| `chore:` | Tooling / maintenance |
| `perf:` | Performance improvement |

Examples:

```text
feat: add repository selection page
feat: add github-style commit timeline
fix: correct commit size formatting
style: refine commit detail drawer
docs: update feature progress
```

A feature should normally be checked off **after** its corresponding commit is ready.

---

# 15. Frontend Architecture Guidelines

Suggested structure:

```text
frontend/
├── src/
│   ├── assets/
│   ├── components/
│   │   ├── common/
│   │   ├── repository/
│   │   ├── commit/
│   │   ├── objects/
│   │   └── cleanup/
│   ├── pages/
│   ├── stores/
│   ├── composables/
│   ├── services/
│   ├── types/
│   ├── utils/
│   └── styles/
```

Suggested responsibilities:

```text
pages
    ↓
stores / composables
    ↓
services
    ↓
Wails binding
```

Avoid calling Wails APIs directly from many unrelated Vue components.

Use a service layer so frontend UI remains testable and backend API changes remain localized.

---

# 16. UX Safety Rules

Because this application deals with destructive Git history operations:

- Read-only actions should look normal.
- History-rewriting actions should use warning styling.
- Force-push actions must be clearly marked dangerous.
- Never execute cleanup from a list row with a single click.
- Always show affected commits/branches/tags first.
- Recommend a backup before rewrite.
- Prefer `--force-with-lease` over unqualified force push.
- Explain that rewriting shared history affects collaborators.
- The application should distinguish:
  - analyze;
  - select;
  - preview;
  - generate;
  - execute.

These stages should not be visually collapsed into one action.

---

# 17. Definition of Done

A feature is considered **Done** only when:

- [ ] Implementation is complete.
- [ ] UI is visually coherent.
- [ ] Main happy path works.
- [ ] Loading state exists.
- [ ] Empty state exists.
- [ ] Error state exists.
- [ ] No known critical regression.
- [ ] Backend dependency is satisfied or explicitly documented.
- [ ] Feature checklist is updated.
- [ ] Progress table is updated.
- [ ] Change log is updated.
- [ ] Completion report is provided to the owner.

---

# 18. Agent Startup Instructions

At the beginning of every coding session, the Agent should:

1. Read this document.
2. Read the current progress table.
3. Check unfinished / blocked features.
4. Check Backend Request records.
5. Inspect only the code needed for the requested task.
6. Confirm whether the task is frontend-only or requires backend coordination.
7. Never modify Go backend code unless explicitly authorized.
8. Work on the smallest coherent feature.
9. Update this document after completion.
10. Report the updated overall percentage.

---

# 19. Agent Final Rule Summary

> **Primary responsibility:** Frontend.

> **Secondary responsibility:** Identify and explain backend changes needed for frontend integration.

> **Backend implementation:** Forbidden unless explicitly authorized by the owner.

> **Progress tracking:** Mandatory.

> **One finished feature = update checkbox + update percentage + report completion.**

> **Do not mark a feature Done simply because the UI exists; integration and basic states must also be accounted for.**

---

# 20. Immediate Recommended Development Order

Recommended MVP order:

- [ ] 1. Project shell
- [ ] 2. Repository picker
- [ ] 3. Repository summary
- [ ] 4. Commit timeline
- [ ] 5. Commit detail panel
- [ ] 6. Introduced size display
- [ ] 7. Changed-file list
- [ ] 8. Large historical objects page
- [ ] 9. Cleanup multi-selection
- [ ] 10. Rewrite preview
- [ ] 11. Command generation
- [ ] 12. UI polish
- [ ] 13. MVP release

The Agent should generally avoid jumping ahead unless the owner explicitly changes priority.

---

**Document status:** Active  
**Initial project completion:** `0%`
