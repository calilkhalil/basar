# Contributing to basar

Thank you for your interest in contributing to basar! This document provides guidelines and instructions for contributing.

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/your-username/basar.git`
3. Create a branch: `git checkout -b feature/your-feature-name`
4. Make your changes
5. Test your changes: `make test`
6. Ensure code is formatted: `make fmt`
7. Commit your changes: `git commit -m "Add: your feature description"`
8. Push to your fork: `git push origin feature/your-feature-name`
9. Open a Pull Request

## Development Setup

```sh
# Build the project
make build

# Run tests
make test

# Format code
make fmt

# Lint code (if golangci-lint is installed)
make lint
```

## Code Style

- Follow Go conventions and idioms
- Use `gofmt` for formatting
- Write clear, descriptive commit messages
- Add comments for exported functions and types
- Keep functions focused and small

## Commit Messages

Use clear, descriptive commit messages:
- `Add: feature description` for new features
- `Fix: bug description` for bug fixes
- `Update: what was updated` for updates
- `Refactor: what was refactored` for refactoring
- `Docs: what documentation was added/changed`

## Pull Request Process

1. Ensure your code follows the style guidelines
2. Add tests if applicable
3. Update documentation if needed
4. Ensure all tests pass
5. Describe your changes clearly in the PR description
6. Reference any related issues

## Reporting Issues

When reporting issues, please include:
- Description of the problem
- Steps to reproduce
- Expected behavior
- Actual behavior
- Environment (OS, Go version, etc.)
- Relevant logs/output

## Questions?

Feel free to open an issue for questions or discussions about the project.

