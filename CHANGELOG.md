# Changelog

All notable changes to this project will be documented in this file.

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
