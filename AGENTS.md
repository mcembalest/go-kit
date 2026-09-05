# Scope

- go-kit is the owner's personal toolkit. Friends install Go and individual apps.
- Keep it to understandable recipes and copyable templates. Do not add a go-kit
  executable, shared runtime dependency, generator, or package manager without need.
- Product behavior and adopted configuration belong in the product repositories.
  Propose concrete integration changes before modifying those repositories.
- Preserve optional updates and exact-version installs; updates are off by default.
- Prefer Go, TypeScript for browser work, and uv-managed Python where appropriate.
- Use established build/release tools. Do not push or publish without authorization.
