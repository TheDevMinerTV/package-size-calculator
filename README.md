# Package Size Calculator

A simple tool to calculate the size of an NPM package. This tool installs the package in a Docker container to ensure that any potentially malicious packages do not affect your system.

## Installation

You can download the pre-built binaries from the [Releases page](https://github.com/TheDevMinerTV/package-size-calculator/releases).

## Prerequisites

> :warning: `package-size-calculator` requires a running Docker daemon.

### Installing Docker

Follow the official Docker installation guides for your platform:

- [Docker Desktop for Windows](https://docs.docker.com/desktop/install/windows-install/)
- [Docker Desktop for Mac](https://docs.docker.com/desktop/install/mac-install/)
- [Docker Engine for Linux](https://docs.docker.com/desktop/install/linux-install/#generic-installation-steps)

## Usage

To calculate the size of an NPM package, run:

```bash
package-size-calculator <package-name>
```

Replace `<package-name>` with the name of the NPM package you want to analyze.

### Additional Flags

- `--short`: Prints a shorter version of the package report, ideal for social media posts.
- `--no-cleanup`: Prevents the removal of the temporary directory after the calculation.
- `--npm-cache <DIRECTORY>`: Specifies a directory to use as the NPM cache. Defaults to a temporary directory if not specified.
- `--npm-cache-read-write`: Mounts the NPM cache directory as read-write. Defaults to true and is only honored if `--npm-cache` is specified.

## Development

### Building from Source

To build the project from source, clone the repository and use the following commands:

```bash
go build -o package-size-calculator
```

## Contributing

Contributions are welcome! Please open an issue or submit a pull request for any improvements or bug fixes.
