# Contributing

PatchBase is open source and we welcome contributions. This guide covers the basics of getting your changes merged.

## Before you start

If you're planning a significant change (new feature, refactoring, breaking change), please open an issue first to discuss it. This avoids wasted work if the change doesn't fit the project's direction.

For bug fixes and small improvements, feel free to open a pull request directly.

## Development workflow

1. **Fork the repository** and clone your fork
2. **Create a branch** for your change:
   ```bash
   git checkout -b fix-some-bug
   ```
3. **Make your changes** following the code conventions below
4. **Run the linter**:
   ```bash
   golangci-lint run
   ```
5. **Run tests**:
   ```bash
   bazel test //...
   ```
6. **Commit and push** your branch
7. **Open a pull request** on the `patchbase/patchbase` repository

## Code conventions

- **Error handling:** wrap errors with `fmt.Errorf("...: %w", err)`
- **Filesystem abstraction:** use `afero.Fs` when an abstraction is needed — don't add a local `fs` wrapper package
- **Unused parameters:** name them `_` in the signature, not `_ = param` in the body
- **Testing:** use `testify/assert` and `testify/require`, not `t.Fatalf`
- **Build files:** after changing Go imports, run `bazel run //:gazelle` to regenerate `BUILD.bazel`
- **New tests:** if Gazelle adds a `go_test` target, annotate it with `size = "small"`
- **No trailing blank lines** at the end of files

## Pull request review

All PRs are reviewed for correctness, security, and code quality. Here's what reviewers look for:

- Does the change solve the problem it claims to?
- Are there security implications (especially around auth, SSH, or credentials)?
- Does it follow the existing code style and conventions?
- Are there tests for new functionality?
- Does it handle errors properly?

## Reporting issues

If you find a bug, please open an issue with:

1. A clear title and description
2. Steps to reproduce
3. Expected vs. actual behavior
4. Relevant logs or error messages
5. Your OS, PatchBase version, and host setup

## License

PatchBase is licensed under the [GNU AGPL-3.0](https://github.com/patchbase/patchbase/blob/main/LICENSE). By contributing, you agree that your contributions will be licensed under the same license.