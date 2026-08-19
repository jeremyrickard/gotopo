package gotopo

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// CredentialsFromINI reads a caltopo_python-compatible account section.
func CredentialsFromINI(path, account string) (Credentials, error) {
	f, err := os.Open(path)
	if err != nil {
		return Credentials{}, fmt.Errorf("gotopo: open credentials file: %w", err)
	}
	defer f.Close()

	values := make(map[string]string)
	active := false
	found := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			active = strings.TrimSpace(line[1:len(line)-1]) == account
			found = found || active
			continue
		}
		if !active {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return Credentials{}, fmt.Errorf("gotopo: malformed credentials line %q", line)
		}
		values[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(value)
	}
	if err := scanner.Err(); err != nil {
		return Credentials{}, fmt.Errorf("gotopo: read credentials file: %w", err)
	}
	if !found {
		return Credentials{}, fmt.Errorf("gotopo: account section %q not found", account)
	}
	credentials := Credentials{
		ID: values["id"], Key: values["key"], AccountID: values["accountid"],
		InternetAccountID: values["accountidinternet"],
	}
	if credentials.ID == "" || credentials.Key == "" {
		return Credentials{}, fmt.Errorf("gotopo: account section %q must contain id and key", account)
	}
	return credentials, nil
}
