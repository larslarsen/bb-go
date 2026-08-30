# Grok Build Handoff — BBGO-SEC-002

You are **Sr Dev — Grok Build**, using Grok 4.6 High. This file is the complete durable
prompt. Ephemeral chat is not authoritative.

Repository: `/home/lars/OpenBazaar/bb-go`

Source-content baseline: `f62d7879fca86db6080ea2ce4e83f6709f1351eb`

Read completely before editing:

1. `AGENTS.md`
2. `TESTING.md`
3. `docs/engineering/DEVELOPMENT_ROLES.md`
4. `docs/security/DEPENDABOT_TRIAGE_2026-08-29.md`
5. `tickets/BBGO-SEC-002.md`

Implement exactly `BBGO-SEC-002` in the shared worktree. First author only
`scripts/legacy_qa_retirement_test.py` as specified. Then delete the complete tracked
`qa/` tree. Do not modify any other path.

Do not run tests or acceptance commands. Do not install dependencies. Do not edit
governance, tickets, handoff, or evidence files. Do not use Git, commit, or push. Do not
replace Requests, preserve marketplace fixtures, or widen deletion to any other legacy
tree.

When finished, stop and report only:

- paths added, modified, and deleted;
- SHA-256 and line count of the new test;
- deleted tracked-file count and diff byte/count summary if available without Git;
- a concise explanation of why the test is non-vacuous; and
- confirmation that no command, dependency install, Git operation, or out-of-scope edit
  occurred.

## Delivered Source Report

Execution date: 2026-08-29

The first invocation had Bash disabled and made no filesystem change. The second
invocation had ordinary filesystem access but again returned after analysis with no
filesystem change. Neither attempt constituted a source drop. The third invocation
explicitly enabled `Read`, `Glob`, `Grep`, `Write`, `Edit`, and `Bash` while retaining the
handoff prohibition on tests, installs, and Git; it completed the following drop:

- Added `scripts/legacy_qa_retirement_test.py` with one test,
  `test_inherited_qa_tree_is_absent`.
- Deleted the complete `qa/` tree: 90 files and five directories, including
  `qa/requirements.txt` and its `requests==2.20.0` pin.
- Reported new-test SHA-256:
  `4ac04254c4b64f542e9ccfb8f736029b5afa2e6339363ab8a7061520a57be183`.
- Reported new-test size: 26 lines and 862 bytes.
- Reported deleted file-content size: 552,042 bytes. A Git diff byte summary was not
  available because the source actor correctly did not use Git.

Grok described the test as repository-root-aware and non-vacuous: it enumerates the real
`qa/` path with `pathlib`, fails if the directory or any descendant remains, and names
the residual paths.

Grok confirmed that it ran no test or acceptance command, installed no dependency, used
no Git operation, edited no out-of-scope path, added no Requests replacement, preserved
no marketplace fixture, and deleted no other legacy tree.
