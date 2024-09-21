# project-indexer

---

check if a project has any changed source files

## Usage

- `project-indexer index <path>` - index a project's source files and write the index to a file
- `project-indexer check <path>` - check if a project has any changed source files, must have been indexed with `index` first
- `project-indexer has-changes <path>` - check if a project has any changed source files; exit code 0 if there are no changes, 1 if there are changes

## Setup

```bash
go mod tidy
```

## Building the project

```bash
task build
```

---

## Changelog

Please see [CHANGELOG](CHANGELOG.md) for more information on what has changed recently.

## Contributing

Please see [CONTRIBUTING](.github/CONTRIBUTING.md) for details.

## Security Vulnerabilities

Please review [our security policy](../../security/policy) on how to report security vulnerabilities.

## Credits

- [Patrick Organ](https://github.com/patinthehat)
- [All Contributors](../../contributors)

## License

The MIT License (MIT). Please see [License File](LICENSE) for more information.
