# AI Agent Attribution & GitHub Guidelines

This repository uses dedicated GitHub App bots for AI coding assistants (**Antigravity**, **Codex**, and **Claude Code**) so that all comments, reviews, and commits are explicitly attributed to the respective agent rather than the repository owner.

---

## 1. GitHub Comments, Issues, and PR Reviews

Whenever interacting with GitHub via the GitHub CLI (`gh`), **never post using the default personal account**. Always execute commands through the `gh-bot` wrapper or pass the agent's bot token via `GH_TOKEN`:

### Using the `gh-bot` CLI Wrapper
The `gh-bot` wrapper is available in `$PATH` (`~/.gemini/antigravity/bin/gh-bot` and `~/.local/bin/`):

- **If you are Antigravity:**
  ```bash
  gh-bot antigravity pr comment <PR> --body "..."
  gh-bot antigravity pr review <PR> --comment --body "..."
  ```
- **If you are Codex:**
  ```bash
  gh-bot codex pr comment <PR> --body "..."
  gh-bot codex pr review <PR> --comment --body "..."
  ```
- **If you are Claude:**
  ```bash
  gh-bot claude pr comment <PR> --body "..."
  gh-bot claude pr review <PR> --comment --body "..."
  ```

### Or using `GH_TOKEN` directly:
```bash
GH_TOKEN="$(gh-bot-token <antigravity|codex|claude>)" gh pr comment <PR> --body "..."
```

---

## 2. Git Commits

When creating Git commits, always specify the `--author` flag matching your agent's GitHub App bot identity. This ensures GitHub renders the commit with the verified bot avatar and `[bot]` badge:

### Commit Author Flags
- **Antigravity:**
  ```bash
  git commit --author="yerassyl-antigravity[bot] <4857259+yerassyl-antigravity[bot]@users.noreply.github.com>" -m "..."
  ```
- **Codex:**
  ```bash
  git commit --author="yerassyl-codex[bot] <4857309+yerassyl-codex[bot]@users.noreply.github.com>" -m "..."
  ```
- **Claude:**
  ```bash
  git commit --author="yerassyl-claude[bot] <4857326+yerassyl-claude[bot]@users.noreply.github.com>" -m "..."
  ```

### Git Trailers (Co-Authorship)
When co-authoring changes or pairing with the human owner, include the trailer in the commit message:
```text
Co-authored-by: yerassyl-antigravity[bot] <4857259+yerassyl-antigravity[bot]@users.noreply.github.com>
Co-authored-by: yerassyl-codex[bot] <4857309+yerassyl-codex[bot]@users.noreply.github.com>
Co-authored-by: yerassyl-claude[bot] <4857326+yerassyl-claude[bot]@users.noreply.github.com>
```

---

## 3. Bot Identities Summary

| Agent | App ID | GitHub Bot Login | Bot Email |
|-------|--------|------------------|-----------|
| **Antigravity** | `4857259` | `yerassyl-antigravity[bot]` | `4857259+yerassyl-antigravity[bot]@users.noreply.github.com` |
| **Codex** | `4857309` | `yerassyl-codex[bot]` | `4857309+yerassyl-codex[bot]@users.noreply.github.com` |
| **Claude** | `4857326` | `yerassyl-claude[bot]` | `4857326+yerassyl-claude[bot]@users.noreply.github.com` |
