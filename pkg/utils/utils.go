package utils

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"openshift-qemu/pkg/logging"
)

// VerifyContinue
// CheckDependencies if we can continue based on user input
func VerifyContinue(yes bool, notes ...string) {
	if yes {
		return
	}

	fmt.Println()
	for _, note := range notes {
		fmt.Println("[NOTE]", note)
	}

	fmt.Print("Press [Enter] to continue, [Ctrl]+C to abort: ")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if strings.ToLower(input) == "n" {
		os.Exit(1)
	}
}

// download a file from a URL and store it in the cache
func download(file, url string, cacheDir string, freshDownload bool) error {
	if file == "" || url == "" {
		logging.Fatal("missing parameters for downloading or verification",
			fmt.Errorf("must have file: '%s', and url '%s'", file, url))
	}

	filePath := filepath.Join(cacheDir, file)
	err := os.MkdirAll(cacheDir, 0o755)
	if err != nil {
		return err
	}

	if _, err = os.Stat(filePath); err == nil {
		fmt.Printf("(reusing cached file %s)\n", file)
	} else {
		err = ValidateURL(url)
		if err != nil {
			logging.Fatal(fmt.Sprintf("%s not reachable", url), err)
		} else {
			logging.Ok("URL is reachable")
		}
	}

	if freshDownload {
		err = os.Remove(filePath)
		if err != nil {
			return err
		}
	}

	if _, err = os.Stat(filePath); err == nil {
		fmt.Printf("(reusing cached file %s)\n", file)
	} else {
		fmt.Println("Downloading file:", file)
		err = downloadFile(url, filePath)
		if err != nil {
			logging.Fatal(fmt.Sprintf("Error downloading %s from %s", file, url), err)
		}
	}

	return nil
}

// ValidateURL checks if a file is downloadable
func ValidateURL(url string) error {
	resp, err := http.Head(url)
	if err != nil || resp.StatusCode != 200 {
		logging.Error(fmt.Sprintf("Failed to download URL: %s", url), err)
		return err
	}
	logging.Info(fmt.Sprintf("File is downloadable: %s\n", url))
	return nil
}

// download a file from a URL
func downloadFile(url, filePath string) error {
	out, err := os.Create(filePath + ".part")
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	// Rename temp file to final name
	return os.Rename(filePath+".part", filePath)
}

// CreateDirectory ensures the setup directory exists and is usable
func CreateDirectory(setupDir string) error {
	err := os.MkdirAll(setupDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create setup directory: %v", err)
	}
	return os.Chdir(setupDir)
}

func extractFile(fileName, cacheDir string) error {
	cmd := exec.Command("tar", "-xf", filepath.Join(cacheDir, fileName))
	err := cmd.Run()
	if err != nil {
		logging.Error(fmt.Sprintf("Failed to extract %s", fileName), err)
		return err
	}
	logging.Ok()
	return nil
}

// Helper functions to handle file creation, downloads, and writing
func touchFile(filePath string) error {
	_, err := os.Create(filePath)
	return err
}

func writeFile(filePath, content string) error {
	return os.WriteFile(filePath, []byte(content), 0o644)
}
