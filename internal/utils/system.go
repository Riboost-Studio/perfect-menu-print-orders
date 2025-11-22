package utils

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// SystemInfo holds information about the current system
type SystemInfo struct {
	OS              string
	Architecture    string
	ChromePresent   bool
	ChromePath      string
}

// DetectSystem returns information about the current operating system and architecture
func DetectSystem() SystemInfo {
	return SystemInfo{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
	}
}

// --------------------------------------
// CHROME CHECK
// --------------------------------------

// CheckChrome checks if google-chrome or chromium is installed
func CheckChrome() (bool, string) {
	// Try common binary names
	binaries := []string{
		"google-chrome",
		"google-chrome-stable",
		"chromium",
		"chromium-browser",
	}

	for _, bin := range binaries {
		path, err := exec.LookPath(bin)
		if err == nil {
			return true, path
		}
	}

	// Check common installation paths
	for _, path := range getCommonChromePaths() {
		if _, err := os.Stat(path); err == nil {
			return true, path
		}
	}

	return false, ""
}

// getCommonChromePaths returns common Chrome/Chromium installation paths
func getCommonChromePaths() []string {
	switch runtime.GOOS {
	case "darwin": // macOS
		return []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		}

	case "linux":
		return []string{
			"/usr/bin/google-chrome",
			"/usr/bin/google-chrome-stable",
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
			"/snap/bin/chromium",
		}

	case "windows":
		return []string{
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files\Chromium\Application\chromium.exe`,
			`C:\Program Files (x86)\Chromium\Application\chromium.exe`,
		}

	default:
		return []string{}
	}
}

// --------------------------------------
// VALIDATION
// --------------------------------------

func ValidateSystemRequirements() error {
	sysInfo := DetectSystem()

	fmt.Printf("System Information:\n")
	fmt.Printf("  OS: %s\n", sysInfo.OS)
	fmt.Printf("  Architecture: %s\n", sysInfo.Architecture)
	fmt.Println()

	// Check for Chrome/Chromium
	isPresent, path := CheckChrome()
	sysInfo.ChromePresent = isPresent
	sysInfo.ChromePath = path

	if isPresent {
		fmt.Printf("✓ Chrome/Chromium found at: %s\n", path)
		fmt.Printf("  Version: %s\n\n", getChromeVersion(path))
		return nil
	}

	// Chrome not found
	fmt.Println("✗ Chrome / Chromium not found!")
	fmt.Println("  It is required for PDF generation using headless Chrome.")
	fmt.Println()

	showChromeInstallationInstructions(sysInfo.OS)

	return fmt.Errorf("chrome/chromium is required but not installed")
}

// getChromeVersion attempts to get the version of Chrome/Chromium
func getChromeVersion(path string) string {
	cmd := exec.Command(path, "--version")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	version := strings.TrimSpace(string(output))
	return version
}

// --------------------------------------
// INSTALLATION INSTRUCTIONS
// --------------------------------------

func showChromeInstallationInstructions(osType string) {
	fmt.Println("Installation Instructions:\n")

	switch osType {

	case "linux":
		fmt.Println("Ubuntu / Debian:")
		fmt.Println("  sudo apt update")
		fmt.Println("  sudo apt install chromium-browser")
		fmt.Println()
		fmt.Println("Fedora:")
		fmt.Println("  sudo dnf install chromium")
		fmt.Println()
		fmt.Println("Arch:")
		fmt.Println("  sudo pacman -S chromium")
		fmt.Println()
		fmt.Println("Google Chrome:")
		fmt.Println("  https://www.google.com/chrome/")

	case "darwin": // macOS
		fmt.Println("Using Homebrew:")
		fmt.Println("  brew install --cask google-chrome")
		fmt.Println()
		fmt.Println("Or Chromium:")
		fmt.Println("  brew install chromium")

	case "windows":
		fmt.Println("Download Google Chrome:")
		fmt.Println("  https://www.google.com/chrome/")
		fmt.Println()
		fmt.Println("Or install Chromium manually.")

	default:
		fmt.Println("Please install Chrome or Chromium for your OS.")
	}

	fmt.Println()
	fmt.Println("After installation, restart this application.")
}
