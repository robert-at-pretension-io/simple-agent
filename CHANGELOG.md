# Changelog

All notable changes to this project will be documented in this file.

## [v1.1.38]

### Fixed
- **CI/CD**: Refactored the GitHub Actions release workflow to eliminate a race condition ("Too many retries" error). The workflow now builds binaries in parallel jobs, collects them, and then creates the release in a single, atomic final step.

## [v1.1.37]

### Fixed
- **Auto-Update**: Implemented a version check before attempting an update. The agent now queries GitHub APIs to see if a new version exists. If the current version matches the latest, the update process is skipped. This prevents infinite update loops and eliminates 404 errors when release assets are missing for the *current* version.

## [v1.1.36]

### Chore
- **CI/CD**: Disabled dependency caching in GitHub Actions. Since the agent uses only the Go standard library (no external deps), `go.sum` is not generated, causing false positive warnings in the build logs.

## [v1.1.35]

### Fixed
- **Build**: Fixed a compile error ("out declared and not used") in the new `autoUpdate` fallback logic. This restores successful builds in the CI/CD pipeline.

## [v1.1.34]

### Fixed
- **CI/CD**: Fixed the GitHub Actions workflow to ensure release binaries are correctly uploaded. Upgraded to `action-gh-release@v2` and improved file matching logic.

## [v1.1.33]

### Fixed
- **Critical**: Removed aggressive "migration cleanup" logic that caused the agent to delete itself after an in-place update when running from the `go/bin` directory. This ensures `go install` remains a first-class, supported installation method.

## [v1.1.32]

### Fixed
- **Update Logic**: Added a fallback mechanism to `autoUpdate`. If the binary update via `install.sh` fails, the agent now attempts to update using `go install` (legacy method). This ensures compatibility for users migrating from older versions or those on unsupported architectures.
- **Self-Update**: Fixed the `install.sh` invocation to correctly pass the current executable path, allowing for in-place updates of the running binary.

## [v1.1.31]

### Improved
- **UX**: The Welcome message (REPL banner) now explicitly includes the Agent Version and Model Name. This ensures the version is visible even if the initial log output scrolls off the screen.

## [v1.1.30]

### Added
- **UX**: The agent now prints its version (e.g., "Simple Agent v1.1.30") immediately on startup. This ensures users always know which version they are running, even before any logs or errors appear.

## [v1.1.25] - 2025-12-07

### Improved
- **UX**: Updated the welcome message to explicitly mention `/help` and `/clear` commands, improving feature discoverability for new users.

### Fixed
- **Stability**: Implemented safety check for `run_script` outputs. Outputs larger than 50,000 characters are now automatically diverted to a file in `.simple_agent/outputs/` to prevent API token limit errors and context overflow.

## [v1.1.24] - 2025-12-07

### Improved
- **System Prompt**: Added a dedicated "**PROJECT MEMORY**" section to the system prompt, explicitly instructing the AI to prioritize reading and updating `remember.txt` using the `remember` skill.

## [v1.1.23] - 2025-12-07

### Improved
- **System Prompt**: Added explicit "How to Include Context" instructions to the `apply_udiff` tool guidelines to reduce common context-related errors.

## [v1.1.12] - 2025-12-07

### Refactor
- **Core Toolset**:
    - Removed `read_file` and `list_files` native tools in favor of `yolo-runner` (shell access) to simplify the agent's capabilities.
    - Updated System Prompt to explicitly encourage using `yolo-runner` for file system operations.
    - Updated `run_script` tool description to emphasize its role as the primary OS interface.

## [v1.1.11] - 2025-12-06

### Maintenance
- Bumped version for release testing.

## [v1.1.10] - 2025-12-06

### Fixed
- **Background Manager Reliability**:
    - Fixed data loss risk in `manager_utils.py` where a wrong password could wipe the state file.
    - Fixed optimistic state in `start_process.py` to ensure processes actually start before being recorded.
    - Fixed potential OOM crash in `get_logs.py` by streaming large logs instead of loading them entirely into RAM.
    - Fixed hanging behavior in `send_input.py` by using non-blocking checks for the input FIFO.
- **Documentation**: Added technical architecture details to `background-manager/SKILL.md`.

## [v1.1.9] - 2025-12-06

### Removed
- **Search Files Tool**: Removed the `search_files` tool from the core agent to streamline functionality.

## [v1.1.8] - 2025-12-06

### Changed
- **Smart Local Updates**: The agent now detects if it is running in a local development environment (source directory) and rebuilds itself using `go build` instead of fetching from the remote repository. This streamlines the local development workflow.

## [v1.1.7] - 2025-12-06

### Added
- **Background Manager Skill**: New skill `background-manager` for running detached processes with persistent state, encryption (AES-256-CBC), and log management.

## [v1.1.6] - 2025-12-06

### Added
- **Web Browser Skill**: New skill `web-browser` with capabilities to search the web (Brave Search) and browse/scrape pages (ScrapingBee).

### Fixed
- **Search Timeout**: Added a 30-second timeout to the `search_files` tool to prevent it from hanging indefinitely on large directories or slow file systems.

## [v1.1.5] - 2025-12-06

### Fixed
- Fixed infinite restart loop when auto-update reinstalls the same version.

## [v1.1.4] - 2025-12-06

### Changed
- **Auto-Update Behavior**: The agent now exits immediately if an update is successfully installed, forcing a restart to ensure the new version is used.

## [v1.1.3] - 2025-12-06

### Changed
- Changed default behavior to **auto-accept** file differences. Use `--no-auto-accept` to restore manual confirmation behavior.

## [v1.1.2] - 2025-12-06

### Added
- Auto-update at startup: The agent now automatically runs `go install ...@latest` on startup to ensure it is up to date. Use `--no-update` to skip.

## [v1.1.1] - 2025-12-06

### Fixed
- Universal fix for `skills/` path resolution. Now works for `read_file`, `list_files`, and `run_script` by handling mapping in the central path validator.

## [v1.1.0] - 2025-12-06

### Added
- Added `--version` flag to check the installed version.
- Added path resolution logic to automatically map `skills/` to the global core skills directory when running from a binary installation.

### Fixed
- Fixed regression where `todo-manager` would freeze by scanning the entire home directory in non-git environments.
- Fixed "file not found" errors when running skills on a machine where the agent was installed via `go install`.

## [v1.0.0] - 2024-05-23

### Added
- Initial release of Simple Agent.
- Core skills: `git-wizard`, `todo-manager`, `yolo-runner`.
- Self-contained binary with embedded skills.
