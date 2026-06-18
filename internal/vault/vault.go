package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// VaultPath — базовый путь к vault
var VaultPath = "/root/LabDoctorM/vault"

// GetKey — прочитать ключ из vault по имени провайдера и имени ключа
func GetKey(provider, keyName string) (string, error) {
	path := filepath.Join(VaultPath, provider, keyName)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("vault: key not found: %s/%s: %w", provider, keyName, err)
	}
	return strings.TrimSpace(string(data)), nil
}

// GetDefaultKey — прочитать первый доступный ключ провайдера
func GetDefaultKey(provider string) (string, error) {
	dir := filepath.Join(VaultPath, provider)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("vault: provider dir not found: %s: %w", provider, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		return strings.TrimSpace(string(data)), nil
	}
	return "", fmt.Errorf("vault: no keys found for provider: %s", provider)
}

// ListProviders — список провайдеров с ключами в vault
func ListProviders() ([]string, error) {
	entries, err := os.ReadDir(VaultPath)
	if err != nil {
		return nil, err
	}
	var providers []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			providers = append(providers, entry.Name())
		}
	}
	return providers, nil
}

// HasKey — проверить наличие ключа
func HasKey(provider, keyName string) bool {
	path := filepath.Join(VaultPath, provider, keyName)
	_, err := os.Stat(path)
	return err == nil
}
