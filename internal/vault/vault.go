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

// GetKeyByPath — прочитать конкретный файл ключа напрямую по абсолютному пути.
// Путь ОБЯЗАН находиться внутри VaultPath (защита от path traversal / SSRF-чтения
// произвольных файлов на диске). Возвращает очищенный (trim) секрет.
func GetKeyByPath(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("vault: path must be absolute: %s", path)
	}
	clean := filepath.Clean(path)
	root := filepath.Clean(VaultPath)
	if clean != root && !strings.HasPrefix(clean, root+string(filepath.Separator)) {
		return "", fmt.Errorf("vault: path outside vault: %s", path)
	}
	data, err := os.ReadFile(clean)
	if err != nil {
		return "", fmt.Errorf("vault: key not found: %s: %w", path, err)
	}
	return strings.TrimSpace(string(data)), nil
}
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
