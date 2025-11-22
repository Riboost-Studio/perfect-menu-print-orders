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
	OS                 string
	Architecture       string
	WkhtmltopdfPresent bool
	WkhtmltopdfPath    string
}

// DetectSystem returns information about the current operating system and architecture
func DetectSystem() SystemInfo {
	return SystemInfo{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
	}
}

// CheckWkhtmltopdf checks if wkhtmltopdf is installed and available in the system
func CheckWkhtmltopdf() (bool, string) {
	// Try to find wkhtmltopdf in PATH
	path, err := exec.LookPath("wkhtmltopdf")
	if err == nil {
		return true, path
	}

	// Check common installation paths for different OS
	commonPaths := getCommonWkhtmltopdfPaths()
	for _, path := range commonPaths {
		if _, err := os.Stat(path); err == nil {
			return true, path
		}
	}

	return false, ""
}

// getCommonWkhtmltopdfPaths returns common installation paths for wkhtmltopdf based on OS
func getCommonWkhtmltopdfPaths() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{
			"C:\\Program Files\\wkhtmltopdf\\bin\\wkhtmltopdf.exe",
			"C:\\Program Files (x86)\\wkhtmltopdf\\bin\\wkhtmltopdf.exe",
		}
	case "darwin": // macOS
		return []string{
			"/usr/local/bin/wkhtmltopdf",
			"/opt/homebrew/bin/wkhtmltopdf",
			"/Applications/wkhtmltopdf.app/Contents/MacOS/wkhtmltopdf",
		}
	case "linux":
		return []string{
			"/usr/bin/wkhtmltopdf",
			"/usr/local/bin/wkhtmltopdf",
			"/opt/wkhtmltopdf/bin/wkhtmltopdf",
		}
	default:
		return []string{"/usr/bin/wkhtmltopdf", "/usr/local/bin/wkhtmltopdf"}
	}
}

// ValidateSystemRequirements checks system requirements and provides installation guidance
func ValidateSystemRequirements() error {
	sysInfo := DetectSystem()
	fmt.Printf("System Information:\n")
	fmt.Printf("  OS: %s\n", sysInfo.OS)
	fmt.Printf("  Architecture: %s\n", sysInfo.Architecture)
	fmt.Println()

	// Check for wkhtmltopdf
	isPresent, path := CheckWkhtmltopdf()
	sysInfo.WkhtmltopdfPresent = isPresent
	sysInfo.WkhtmltopdfPath = path

	if isPresent {
		fmt.Printf("✓ wkhtmltopdf found at: %s\n", path)

		// Test wkhtmltopdf version
		if version := getWkhtmltopdfVersion(path); version != "" {
			fmt.Printf("  Version: %s\n", version)
		}

		fmt.Println()
		return nil
	}

	// wkhtmltopdf not found - provide installation instructions
	fmt.Println("✗ wkhtmltopdf not found!")
	fmt.Println("  wkhtmltopdf is required to generate PDF receipts from HTML templates.")
	fmt.Println()

	showInstallationInstructions(sysInfo.OS)

	return fmt.Errorf("wkhtmltopdf is required but not installed")
}

// getWkhtmltopdfVersion attempts to get the version of wkhtmltopdf
func getWkhtmltopdfVersion(path string) string {
	cmd := exec.Command(path, "--version")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	version := strings.TrimSpace(string(output))
	// Extract just the version number (first line usually contains it)
	lines := strings.Split(version, "\n")
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}

	return version
}

// showInstallationInstructions displays OS-specific installation instructions
func showInstallationInstructions(osType string) {
	fmt.Println("Installation Instructions:")
	fmt.Println()

	switch osType {
	case "linux":
		fmt.Println("Ubuntu/Debian:")
		fmt.Println("  sudo apt update")
		fmt.Println("  sudo apt install wkhtmltopdf")
		fmt.Println()
		fmt.Println("CentOS/RHEL/Fedora:")
		fmt.Println("  sudo yum install wkhtmltopdf")
		fmt.Println("  # or for newer versions:")
		fmt.Println("  sudo dnf install wkhtmltopdf")
		fmt.Println()
		fmt.Println("Arch Linux:")
		fmt.Println("  sudo pacman -S wkhtmltopdf")
		fmt.Println()
		fmt.Println("Manual installation:")
		fmt.Println("  Download from: https://wkhtmltopdf.org/downloads.html")

	case "darwin": // macOS
		fmt.Println("Using Homebrew (recommended):")
		fmt.Println("  brew install wkhtmltopdf")
		fmt.Println()
		fmt.Println("Using MacPorts:")
		fmt.Println("  sudo port install wkhtmltopdf")
		fmt.Println()
		fmt.Println("Manual installation:")
		fmt.Println("  Download from: https://wkhtmltopdf.org/downloads.html")

	case "windows":
		fmt.Println("Download and install from:")
		fmt.Println("  https://wkhtmltopdf.org/downloads.html")
		fmt.Println()
		fmt.Println("Choose the Windows installer and follow the setup wizard.")
		fmt.Println("Make sure to add wkhtmltopdf to your system PATH.")

	default:
		fmt.Println("Please visit https://wkhtmltopdf.org/downloads.html")
		fmt.Println("and download the appropriate package for your operating system.")
	}

	fmt.Println()
	fmt.Println("After installation, restart this application to continue.")
}
