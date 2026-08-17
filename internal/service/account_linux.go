package service

import (
	"bufio"
	"os"
	"strings"
)

func parsePasswd() ([]passwdEntry, error) {
	return parsePasswdFile("/etc/passwd")
}

func parsePasswdFile(path string) ([]passwdEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []passwdEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 7 {
			continue
		}
		entries = append(entries, passwdEntry{Name: parts[0], GID: parts[3]})
	}
	return entries, scanner.Err()
}

func parseGroup() ([]groupEntry, error) {
	return parseGroupFile("/etc/group")
}

func parseGroupFile(path string) ([]groupEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []groupEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 4 {
			continue
		}
		var members []string
		if m := strings.TrimSpace(parts[3]); m != "" {
			members = strings.Split(m, ",")
		}
		entries = append(entries, groupEntry{GID: parts[2], Members: members})
	}
	return entries, scanner.Err()
}
