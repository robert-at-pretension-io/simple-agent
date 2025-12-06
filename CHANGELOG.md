# Changelog

All notable changes to this project will be documented in this file.

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
