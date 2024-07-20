# Package size calculator

Simple tool to calculate the size of a NPM package. It installs the package in a Docker container so any malicious packages won't affect your system.

## Using

You can find the built binaries [here](https://github.com/TheDevMinerTV/package-size-calculator/releases).

> :warning: `package-size-calculator` requires having a Docker daemon running.

```bash
package-size-calculator
```

The following additional flags are available:
```
--short      : Print a shorter version of the package report, ideal for posts to Twitter
--no-cleanup : Don't remove the temporary directory after the calculation
```

## License

Package size calculator is licensed under the MIT License. See [LICENSE](LICENSE) for the full license text.
