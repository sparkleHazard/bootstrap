package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	gitHubKeyURL  = "192.168.1.8/keys/id_ecdsa_github"
	repoURL       = "git@github.com:sparkleHazard/ansible.git"
	vaultPassFile = ".vault_pass.txt"
	ansibleSite   = "ansible/site.yml"
	miseCmd       = "/home/linuxbrew/.linuxbrew/bin/mise install"
)

var (
	role           string
	verbose        bool
	runMiseInstall bool
)

func main() {
	// 1. Parse arguments
	flag.StringVar(&role, "role", "base", "Role to use for provisioning (e.g., base, keyserver, webserver).")
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose output.")
	flag.BoolVar(&runMiseInstall, "mise-install", false, "Enable one-shot systemd service for 'mise install' after reboot.")
	flag.Parse()

	log("Starting Go-based bootstrap...")

	// 2. Ensure ~/.ssh directory
	ensureSSHDirectory()

	// 3. Detect OS
	osID := detectOS()
	log(fmt.Sprintf("Detected OS: %s", osID))

	// For macOS, ensure Homebrew is installed.
	if osID == "darwin" {
		ensureHomebrew()
	}

	// 4. Prerequisite checks
	ensureSudo(osID)
	ensureCommandInstalled(osID, "curl")
	ensureCommandInstalled(osID, "git")
	ensureCommandInstalled(osID, "rsync")
	ensureCommandInstalled(osID, "jq")
	ensureAnsible(osID)
	ensureGh(osID)

	// 5. If role == keyserver, handle GitHub key; otherwise, fetch private key via rsync.
	if role == "keyserver" {
		ensureGhAuth()
		manageSSHKeyForGitHub()
	} else {
		fetchGithubPrivateKey()
	}

	// 6. Run ansible-pull
	runAnsiblePull()

	// 7. Optionally set up one-shot systemd service for 'mise install'
	if runMiseInstall {
		setupMiseInstallService()
	} else {
		log("Skipping mise install setup.")
	}

	log("Bootstrapping complete.")
}

// log prints a timestamped message to stdout.
func log(msg string) {
	now := time.Now().Format("2006-01-02 15:04:05")
	fmt.Printf("[%s] %s\n", now, msg)
}

// runCmd runs a command on the host system, streaming its output.
func runCmd(name string, args ...string) error {
	if verbose {
		log(fmt.Sprintf("Running: %s %s", name, strings.Join(args, " ")))
	}
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runCmdSudo wraps runCmd in "sudo" unless we are already root.
func runCmdSudo(name string, args ...string) error {
	if os.Geteuid() != 0 {
		newArgs := append([]string{name}, args...)
		return runCmd("sudo", newArgs...)
	}
	return runCmd(name, args...)
}

// detectOS attempts to read /etc/os-release or check for Darwin.
func detectOS() string {
	if _, err := os.Stat("/System/Library/CoreServices/SystemVersion.plist"); err == nil {
		return "darwin"
	}
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return "unknown"
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "ID=") {
			return strings.Trim(strings.Split(line, "=")[1], `"`)
		}
	}
	return "unknown"
}

// ensureSSHDirectory ensures that ~/.ssh exists, creating it if necessary.
func ensureSSHDirectory() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log("Error: Unable to find home directory.")
		os.Exit(1)
	}
	sshPath := filepath.Join(homeDir, ".ssh")
	if _, err := os.Stat(sshPath); os.IsNotExist(err) {
		log("~/.ssh does not exist; creating...")
		if err := os.MkdirAll(sshPath, 0700); err != nil {
			log("Failed to create ~/.ssh directory: " + err.Error())
			os.Exit(1)
		}
	} else {
		if verbose {
			log("~/.ssh directory already exists.")
		}
	}
}

// ensureHomebrew ensures Homebrew is installed on macOS.
func ensureHomebrew() {
	if _, err := exec.LookPath("brew"); err == nil {
		if verbose {
			log("Homebrew is already installed.")
		}
		return
	}
	log("Homebrew is not installed. Attempting to install Homebrew...")

	// Pre-cache sudo credentials.
	if err := runCmd("sudo", "-v"); err != nil {
		log("Failed to get sudo credentials: " + err.Error())
		os.Exit(1)
	}

	// Run the official Homebrew installer in non-interactive CI mode.
	// Setting both NONINTERACTIVE=1 and CI=1 may help suppress prompts.
	cmd := exec.Command("/bin/bash", "-c", "NONINTERACTIVE=1 CI=1 curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh | /bin/bash")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log("Failed to install Homebrew: " + err.Error())
		log("Please ensure your user has the necessary sudo privileges and try again, or install Homebrew manually.")
		os.Exit(1)
	}
}

// ensureSudo checks if sudo is installed, and attempts to install it if not.
func ensureSudo(osID string) {
	if _, err := exec.LookPath("sudo"); err == nil {
		if verbose {
			log("sudo is already installed.")
		}
		return
	}
	log("sudo not found. Attempting to install...")
	switch osID {
	case "ubuntu", "debian":
		runCmdSudo("apt-get", "update")
		runCmdSudo("apt-get", "install", "-y", "sudo")
	case "fedora":
		runCmdSudo("dnf", "install", "-y", "sudo")
	case "centos", "redhat":
		runCmdSudo("yum", "install", "-y", "sudo")
	case "darwin":
		log("Warning: Installing sudo on macOS via Homebrew (if needed).")
		runCmd("brew", "install", "sudo")
	default:
		log("Unsupported OS for automatic sudo installation. Install sudo manually.")
		os.Exit(1)
	}
}

// ensureCommandInstalled checks if a command is installed and installs it if not.
func ensureCommandInstalled(osID, cmdName string) {
	if _, err := exec.LookPath(cmdName); err == nil {
		if verbose {
			log(cmdName + " is already installed.")
		}
		return
	}
	log(fmt.Sprintf("%s is not installed. Installing...", cmdName))
	switch osID {
	case "ubuntu", "debian":
		runCmdSudo("apt-get", "update")
		runCmdSudo("apt-get", "install", "-y", cmdName)
	case "fedora":
		runCmdSudo("dnf", "install", "-y", cmdName)
	case "centos", "redhat":
		if cmdName == "jq" || cmdName == "rsync" {
			runCmdSudo("yum", "install", "-y", "epel-release")
		}
		runCmdSudo("yum", "install", "-y", cmdName)
	case "darwin":
		runCmd("brew", "install", cmdName)
	default:
		log("Unsupported OS for automatic installation of " + cmdName)
		os.Exit(1)
	}
}

// ensureAnsible checks if ansible-playbook is installed and installs it if not.
func ensureAnsible(osID string) {
	_, err := exec.LookPath("ansible-playbook")
	if err == nil {
		if verbose {
			log("Ansible is already installed.")
		}
		return
	}
	log("Ansible not found. Installing...")
	switch osID {
	case "ubuntu", "debian":
		runCmdSudo("apt-get", "update")
		runCmdSudo("apt-get", "install", "-y", "ansible")
	case "fedora":
		runCmdSudo("dnf", "install", "-y", "ansible")
	case "centos", "redhat":
		runCmdSudo("yum", "install", "-y", "epel-release")
		runCmdSudo("yum", "install", "-y", "ansible")
	case "darwin":
		runCmd("brew", "install", "ansible")
	default:
		log("Falling back to pip-based Ansible installation...")
		runCmd("pip", "install", "--user", "ansible")
	}
}

// ensureGh checks if the GitHub CLI is installed and installs it if not.
func ensureGh(osID string) {
	_, err := exec.LookPath("gh")
	if err == nil {
		if verbose {
			log("GitHub CLI (gh) is already installed.")
		}
		return
	}
	log("GitHub CLI not found. Installing...")
	switch osID {
	case "darwin":
		runCmd("brew", "install", "gh")
	case "ubuntu", "debian":
		if err := runCmdSudo("bash", "-c", "curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg"); err != nil {
			log("Error installing GitHub CLI key: " + err.Error())
			os.Exit(1)
		}
		runCmdSudo("chmod", "go+r", "/usr/share/keyrings/githubcli-archive-keyring.gpg")
		archBytes, err := exec.Command("dpkg", "--print-architecture").Output()
		if err != nil {
			log("Failed to detect architecture.")
			os.Exit(1)
		}
		arch := strings.TrimSpace(string(archBytes))
		debRepoLine := fmt.Sprintf("deb [arch=%s signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main", arch)
		runCmdSudo("bash", "-c", fmt.Sprintf("echo '%s' > /etc/apt/sources.list.d/github-cli.list", debRepoLine))
		runCmdSudo("apt-get", "update")
		runCmdSudo("apt-get", "install", "-y", "gh")
	case "fedora":
		runCmdSudo("dnf", "config-manager", "--add-repo", "https://cli.github.com/packages/rpm/gh-cli.repo")
		runCmdSudo("dnf", "install", "-y", "gh")
	case "centos", "redhat":
		runCmdSudo("yum-config-manager", "--add-repo", "https://cli.github.com/packages/rpm/gh-cli.repo")
		runCmdSudo("yum", "install", "-y", "gh")
	default:
		log("Unsupported OS for GitHub CLI installation. Please install gh manually.")
		os.Exit(1)
	}
}

// ensureGhAuth checks if gh auth status is successful; if not, prompts for a token.
func ensureGhAuth() {
	err := exec.Command("gh", "auth", "status").Run()
	if err == nil {
		if verbose {
			log("GitHub CLI is already authenticated.")
		}
		return
	}
	log("GitHub CLI is not authenticated.")
	fmt.Print("Please enter your GitHub Personal Access Token: ")
	reader := bufio.NewReader(os.Stdin)
	token, _ := reader.ReadString('\n')
	token = strings.TrimSpace(token)
	if token == "" {
		log("No token provided, aborting.")
		os.Exit(1)
	}
	os.Setenv("GH_TOKEN", token)
	err = exec.Command("gh", "auth", "status").Run()
	if err != nil {
		log("GitHub CLI authentication failed even after setting GH_TOKEN. Aborting.")
		os.Exit(1)
	}
}

// manageSSHKeyForGitHub generates an ECDSA SSH key if it doesn't exist and ensures it's registered with GitHub.
func manageSSHKeyForGitHub() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log("Unable to determine home directory.")
		os.Exit(1)
	}

	keyPath := filepath.Join(homeDir, ".ssh", "id_ecdsa_github")
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		log("Generating new ECDSA key pair for GitHub...")
		if err := runCmd("ssh-keygen", "-t", "ecdsa", "-b", "521", "-f", keyPath, "-N", "", "-q", "-C", ""); err != nil {
			log("Failed to generate SSH key: " + err.Error())
			os.Exit(1)
		}
	} else {
		if verbose {
			log("ECDSA key pair already exists at " + keyPath)
		}
	}

	pubBytes, err := os.ReadFile(keyPath + ".pub")
	if err != nil {
		log("Failed to read public key: " + err.Error())
		os.Exit(1)
	}
	publicKey := string(pubBytes)

	// Test SSH access to GitHub using the local key.
	sshTest := exec.Command("ssh", "-T", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=no", "-i", keyPath, "git@github.com")
	out, err := sshTest.CombinedOutput()
	outStr := strings.ToLower(string(out))

	if err == nil && strings.Contains(outStr, "successfully authenticated") {
		log("SSH key is accepted by GitHub.")
		return
	}
	log("SSH key access denied. Attempting to update GitHub keys...")

	// Attempt to remove old key with "keyserver" title.
	delOldCmd := exec.Command("gh", "api", "-H", "Accept: application/vnd.github+json",
		"-H", "X-GitHub-Api-Version: 2022-11-28",
		"/user/keys")
	outList, err := delOldCmd.Output()
	if err == nil {
		keyID := findKeyIDForTitle(string(outList), "keyserver")
		if keyID != "" {
			log("Deleting old GitHub key with ID: " + keyID)
			exec.Command("gh", "api", "--method", "DELETE", "-H", "Accept: application/vnd.github+json",
				"-H", "X-GitHub-Api-Version: 2022-11-28",
				fmt.Sprintf("/user/keys/%s", keyID)).Run()
		}
	}

	log("Adding new SSH key to GitHub...")
	addCmd := exec.Command("gh", "api", "--method", "POST", "-H", "Accept: application/vnd.github+json",
		"-H", "X-GitHub-Api-Version: 2022-11-28",
		"/user/keys", "-f", "key="+publicKey, "-f", "title=keyserver")
	if err := addCmd.Run(); err != nil {
		log("Failed to add new SSH key to GitHub: " + err.Error())
	}
}

// findKeyIDForTitle is a helper to parse JSON from `gh api /user/keys` output
// and return the `.id` for a given `.title`.
func findKeyIDForTitle(jsonStr, title string) string {
	idRegex := regexp.MustCompile(`"id":\s*([0-9]+).*"title":\s*"` + regexp.QuoteMeta(title) + `"`)
	matches := idRegex.FindStringSubmatch(jsonStr)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

// fetchGithubPrivateKey uses rsync to pull the key from some remote location.
func fetchGithubPrivateKey() {
	log("Fetching GitHub SSH private key via rsync...")
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log("Unable to determine home directory.")
		os.Exit(1)
	}
	keyDest := filepath.Join(homeDir, ".ssh", "id_ecdsa_github")

	tmpDest := "/tmp/github_key"
	rsyncSrc := "rsync://" + gitHubKeyURL

	const maxRetries = 5
	const sleepSeconds = 10
	for i := 0; i < maxRetries; i++ {
		err := runCmd("rsync", "-avz", rsyncSrc, tmpDest)
		if err == nil {
			break
		}
		if i == maxRetries-1 {
			log("Error: Unable to fetch GitHub SSH private key after multiple retries.")
			os.Exit(1)
		}
		log(fmt.Sprintf("rsync failed (attempt %d/%d). Retrying in %d seconds...", i+1, maxRetries, sleepSeconds))
		time.Sleep(sleepSeconds * time.Second)
	}

	contentTmp, err := os.ReadFile(tmpDest)
	if err != nil {
		log("Error reading temp GitHub key: " + err.Error())
		os.Exit(1)
	}

	existing, err := os.ReadFile(keyDest)
	if err == nil {
		if string(existing) == string(contentTmp) {
			log("GitHub SSH private key is already up-to-date.")
			return
		}
	}
	if err := os.WriteFile(keyDest, contentTmp, 0600); err != nil {
		log("Error writing GitHub SSH key: " + err.Error())
		os.Exit(1)
	}
	log("GitHub SSH private key updated at " + keyDest)
}

// runAnsiblePull runs ansible-pull with the appropriate key, vault, etc.
func runAnsiblePull() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log("Unable to find home directory for ansible-pull.")
		os.Exit(1)
	}
	keyPath := filepath.Join(homeDir, ".ssh", "id_ecdsa_github")
	vaultPath := filepath.Join(homeDir, vaultPassFile)

	log("Running ansible-pull...")
	args := []string{
		"-U", repoURL,
		"-i", "localhost,",
		"--extra-vars", fmt.Sprintf("host_role=%s", role),
		"--private-key", keyPath,
		"--accept-host-key",
		"--vault-password-file", vaultPath,
		ansibleSite,
	}
	if err := runCmd("ansible-pull", args...); err != nil {
		log("ansible-pull failed: " + err.Error())
		os.Exit(1)
	}
}

// setupMiseInstallService creates a systemd service that runs "mise install" after reboot, then reboots.
func setupMiseInstallService() {
	log("Setting up one-shot systemd service for 'mise install' after reboot...")

	targetUser := os.Getenv("SUDO_USER")
	if targetUser == "" {
		usr, err := user.Current()
		if err != nil {
			log("Cannot determine current user.")
			os.Exit(1)
		}
		targetUser = usr.Username
	}

	u, err := user.Lookup(targetUser)
	if err != nil {
		log("Cannot look up user " + targetUser + ": " + err.Error())
		os.Exit(1)
	}
	targetHome := u.HomeDir

	serviceContent := fmt.Sprintf(`[Unit]
Description=Run mise install once after reboot
After=network.target

[Service]
Type=oneshot
User=%s
Environment=HOME=%s
ExecStart=/bin/zsh -i -c "%s"
ExecStartPost=/bin/systemctl disable mise-install-once.service && /bin/rm -f /etc/systemd/system/mise-install-once.service && /bin/systemctl daemon-reload

[Install]
WantedBy=multi-user.target
`, targetUser, targetHome, miseCmd)

	servicePath := "/etc/systemd/system/mise-install-once.service"

	err = os.WriteFile("/tmp/mise-install-once.service", []byte(serviceContent), 0644)
	if err != nil {
		log("Failed to write temp systemd service file: " + err.Error())
		os.Exit(1)
	}

	if err := runCmdSudo("mv", "/tmp/mise-install-once.service", servicePath); err != nil {
		log("Failed to move service file: " + err.Error())
		os.Exit(1)
	}
	if err := runCmdSudo("systemctl", "daemon-reload"); err != nil {
		os.Exit(1)
	}
	if err := runCmdSudo("systemctl", "enable", "mise-install-once.service"); err != nil {
		os.Exit(1)
	}

	log("One-shot service created and enabled. Rebooting now...")
	if err := runCmdSudo("reboot"); err != nil {
		log("Failed to reboot: " + err.Error())
	}
}
