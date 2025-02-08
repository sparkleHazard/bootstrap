# Bootstrap

Bootstrap is a cross-platform bootstrapping tool written in Go that automates the provisioning and configuration of servers. It replaces the original Bash-based bootstrap script with a more maintainable, robust, and portable Go implementation.

## Features

- **OS-Agnostic Prerequisite Checks:**  
  Automatically verifies and installs required tools (e.g. sudo, curl, Git, rsync, jq, Ansible, GitHub CLI).

- **GitHub CLI Integration:**  
  Installs and authenticates GitHub CLI (`gh`), manages SSH keys for different roles (e.g. keyserver), and updates keys on GitHub as needed.

- **Ansible Integration:**  
  Runs `ansible-pull` with the appropriate SSH key and vault password support, making it easy to bootstrap servers with Ansible-based configurations.

- **One-Shot Post-Reboot Service:**  
  Optionally sets up a one-shot systemd service (using the `--mise-install` flag) that runs a command (e.g. `/home/linuxbrew/.linuxbrew/bin/mise install`) once after reboot.

- **Modular and Extensible:**  
  Written in Go for better error handling, maintainability, and ease of adding new features compared to a complex Bash script.

## Installation

### Prerequisites

- [Go](https://golang.org/dl/) (version 1.16 or later)
- [Git](https://git-scm.com/)

### Building the Binary

Clone the repository and build the executable:

```bash
git clone https://github.com/sparkleHazard/bootstrap.git
cd bootstrap
go build -o bootstrap .
```

This will produce a binary named bootstrap that you can run on your server.

Usage

Bootstrap can be run directly on the target host. It accepts several command-line arguments to control its behavior. For example:

```bash
sudo ./bootstrap --role=webserver --verbose --mise-install
```

### Available Flags

- `--role=ROLE`
  Specify the server role to provision (e.g. base, keyserver, webserver).
  Default: base
- `--verbose`
  Enable verbose output for detailed logging.
- `--mise-install`
  Set up a one-shot systemd service to run /home/linuxbrew/.linuxbrew/bin/mise install once after reboot.
- `--help`
  Display usage information.

### Configuration

Certain configuration options (such as repository URL, vault password file location, and command paths) are defined within the source code as variables. You can adjust these in the main source file as needed for your environment.

### Integration with Ansible

Bootstrap is designed to integrate seamlessly with Ansible:

- It ensures prerequisites are met and the environment is configured.
- It invokes ansible-pull with proper SSH keys and vault support to apply configuration from an Ansible repository.
- It supports both pull (ansible-pull) and push (ansible-playbook) models.

For more details on how to integrate Ansible with your provisioning, see the Ansible documentation.

### Contributing

Contributions are welcome! Please submit issues or pull requests for:

- Bug fixes and improvements
- New features and role enhancements
- Documentation updates

For major changes, please open an issue first to discuss your ideas.

## Release Process - Current release v1.0.2

Our project follows [Semantic Versioning](https://semver.org/). Version numbers follow the format `vMAJOR.MINOR.PATCH`.

- **MAJOR:** Increment when you make incompatible API changes.
- **MINOR:** Increment when you add functionality in a backward-compatible manner.
- **PATCH:** Increment when you make backward-compatible bug fixes.

### Tagging a Release

1. Make sure your local repository is up to date:

   ```bash
   git pull
   ```

1. Tag the current commit:

   ```bash
   git tag -a v1.0.0 -m "Release version 1.0.0"
   ```

1. Push the tag to GitHub:

   ```bash
   git push origin v1.0.0
   ```

### Changelog

Please update the CHANGELOG.md with a summary of changes for each release. We follow [Keep a Changelog](https://keepachangelog.com/en/1.0.0/) format.

### Continuous Integration and Deployment

When a new version tag is pushed, our CI/CD pipeline automatically builds the binaries for all supported platforms and creates a GitHub Release with the release assets and changelog. Refer to our GitHub Actions workflows for further details.

### License

This project is licensed under the MIT License. See the LICENSE file for details.

---

### Final Notes

- **Customization:** Adjust the repository URL, file paths, and any configuration variables in the source as needed.
- **Testing:** Make sure to test the binary in your target environments to ensure it works as expected.
- **Documentation:** Update the README as new features or configuration options are added.

Let me know if you need further modifications or additional details!
