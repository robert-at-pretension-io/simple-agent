# Agent Maintenance Guide

This document outlines common administrative tasks for maintaining the `simple-agent` project.

## Releasing a New Version

To release a new version (e.g., bumping from `v1.1.0` to `v1.1.1`):

1.  **Update Code**:
    - Modify `const Version` in `main.go`.
    - Update the version number in `README.md`.

2.  **Update Changelog**:
    - Add a new entry in `CHANGELOG.md` with the date and list of changes.

3.  **Commit Changes**:
    ```bash
    git add main.go README.md CHANGELOG.md
    git commit -m "chore: release v1.1.1"
    git push
    ```

4.  **Tag the Release**:
    Go modules rely on git tags for versioning. This is crucial for `go install` to report the correct version.
    ```bash
    git tag v1.1.1
    git push origin v1.1.1
    ```

## Deploying Updates

Users can update to the latest version using `go install`.

**Force Update (Bypass Cache):**
If you just pushed a tag, use `GOPROXY=direct` to ensure the Go proxy sees it immediately.

```bash
GOPROXY=direct go install github.com/robert-at-pretension-io/simple-agent@latest
```

## Managing Skills

Skills are embedded into the binary at build time via the `embed` package in `main.go`.

- **Modifying Skills**: Edit files in the local `skills/` directory.
- **Applying Changes**: You must rebuild/reinstall the agent to see changes on a deployed machine.
- **Path Resolution**: The agent automatically maps `skills/` paths to the internal `CoreSkillsDir` for installed binaries. Do not hardcode absolute paths in skill scripts; rely on the `skills/` prefix.
