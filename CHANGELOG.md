# Changelog

## [Unreleased]

### Added
- `model.IsOverdue(due, now)`: pure helper to detect overdue tasks. A zero
  due date (no date) or a future one never appears overdue.
