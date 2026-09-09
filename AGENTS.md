# Scope

- go-kit is a personal Go CLI: init, dev, ship. Apps remain independently installable.
- Keep workflows small and use established tools underneath. No plugin platform,
  shared app runtime dependency, or custom package manager.
- Product behavior belongs in product repositories. Propose concrete integration
  changes before modifying them. Preserve existing code during adoption.
- Updates are opt-in; exact-version installs and rollback remain supported.
- Default to Go delivery/backend and optional TypeScript browser UI. Use uv for
  optional Python tooling; other languages remain app-owned.
- README is a concise table-based command/settings inventory. Keep implementation
  details in DEVELOPMENT.md. Document every public flag and publishing trigger.
- Test with disposable projects. Do not push or publish without authorization.
