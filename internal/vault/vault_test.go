package vault

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestVault(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	origPath := VaultPath
	VaultPath = tmpDir
	t.Cleanup(func() { VaultPath = origPath })
	return tmpDir
}

func TestGetKey(t *testing.T) {
	tmpDir := setupTestVault(t)

	// Создаём тестовый ключ
	provider := "test_provider"
	keyName := "api.key"
	keyValue := "test-api-key-12345"

	providerDir := filepath.Join(tmpDir, provider)
	if err := os.MkdirAll(providerDir, 0700); err != nil {
		t.Fatalf("failed to create provider dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(providerDir, keyName), []byte(keyValue), 0600); err != nil {
		t.Fatalf("failed to write key file: %v", err)
	}

	// Тест: загрузка ключа
	value, err := GetKey(provider, keyName)
	if err != nil {
		t.Fatalf("GetKey failed: %v", err)
	}
	if value != keyValue {
		t.Errorf("expected %q, got %q", keyValue, value)
	}
}

func TestGetKeyNotFound(t *testing.T) {
	setupTestVault(t)

	_, err := GetKey("nonexistent", "api.key")
	if err == nil {
		t.Error("expected error for nonexistent key, got nil")
	}
}

func TestGetDefaultKey(t *testing.T) {
	tmpDir := setupTestVault(t)

	provider := "default_test"
	keyValue := "default-key-value"

	providerDir := filepath.Join(tmpDir, provider)
	if err := os.MkdirAll(providerDir, 0700); err != nil {
		t.Fatalf("failed to create provider dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(providerDir, "api.key"), []byte(keyValue), 0600); err != nil {
		t.Fatalf("failed to write key: %v", err)
	}

	value, err := GetDefaultKey(provider)
	if err != nil {
		t.Fatalf("GetDefaultKey failed: %v", err)
	}
	if value != keyValue {
		t.Errorf("expected %q, got %q", keyValue, value)
	}
}

func TestGetDefaultKeyMultiple(t *testing.T) {
	tmpDir := setupTestVault(t)

	provider := "multi_test"
	providerDir := filepath.Join(tmpDir, provider)
	if err := os.MkdirAll(providerDir, 0700); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	// Создаём несколько ключей — должен вернуться первый
	keys := map[string]string{
		"primary.key":   "primary-value",
		"secondary.key": "secondary-value",
	}
	for name, value := range keys {
		if err := os.WriteFile(filepath.Join(providerDir, name), []byte(value), 0600); err != nil {
			t.Fatalf("failed to write key %s: %v", name, err)
		}
	}

	value, err := GetDefaultKey(provider)
	if err != nil {
		t.Fatalf("GetDefaultKey failed: %v", err)
	}
	// Должен вернуться один из ключей
	if value != "primary-value" && value != "secondary-value" {
		t.Errorf("unexpected key value: %q", value)
	}
}

func TestGetDefaultKeyEmpty(t *testing.T) {
	tmpDir := setupTestVault(t)

	provider := "empty_test"
	providerDir := filepath.Join(tmpDir, provider)
	if err := os.MkdirAll(providerDir, 0700); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	_, err := GetDefaultKey(provider)
	if err == nil {
		t.Error("expected error for provider with no keys, got nil")
	}
}

func TestListProviders(t *testing.T) {
	tmpDir := setupTestVault(t)

	providers := []string{"provider_a", "provider_b", "provider_c"}
	for _, p := range providers {
		dir := filepath.Join(tmpDir, p)
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatalf("failed to create dir for %s: %v", p, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "api.key"), []byte("key"), 0600); err != nil {
			t.Fatalf("failed to write key for %s: %v", p, err)
		}
	}

	// Создаём скрытую директорию — она не должна быть в списке
	hiddenDir := filepath.Join(tmpDir, ".hidden")
	os.MkdirAll(hiddenDir, 0700)

	listed, err := ListProviders()
	if err != nil {
		t.Fatalf("ListProviders failed: %v", err)
	}

	if len(listed) != len(providers) {
		t.Errorf("expected %d providers, got %d", len(providers), len(listed))
	}

	providerMap := make(map[string]bool)
	for _, p := range listed {
		providerMap[p] = true
	}
	for _, p := range providers {
		if !providerMap[p] {
			t.Errorf("provider %s not found in list", p)
		}
	}

	// Скрытая директория не должна быть в списке
	if providerMap[".hidden"] {
		t.Error("hidden directory should not be in providers list")
	}
}

func TestHasKey(t *testing.T) {
	tmpDir := setupTestVault(t)

	provider := "haskey_test"
	providerDir := filepath.Join(tmpDir, provider)
	if err := os.MkdirAll(providerDir, 0700); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(providerDir, "api.key"), []byte("key"), 0600); err != nil {
		t.Fatalf("failed to write key: %v", err)
	}

	tests := []struct {
		provider string
		keyName  string
		expected bool
	}{
		{provider, "api.key", true},
		{provider, "nonexistent.key", false},
		{"nonexistent", "api.key", false},
	}

	for _, tt := range tests {
		result := HasKey(tt.provider, tt.keyName)
		if result != tt.expected {
			t.Errorf("HasKey(%q, %q) = %v, want %v", tt.provider, tt.keyName, result, tt.expected)
		}
	}
}

func TestKeyPermissions(t *testing.T) {
	tmpDir := setupTestVault(t)

	provider := "perms_test"
	providerDir := filepath.Join(tmpDir, provider)
	if err := os.MkdirAll(providerDir, 0700); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	keyPath := filepath.Join(providerDir, "api.key")
	if err := os.WriteFile(keyPath, []byte("secret-key"), 0600); err != nil {
		t.Fatalf("failed to write key: %v", err)
	}

	// Проверяем права на файл
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("failed to stat key file: %v", err)
	}

	mode := info.Mode().Perm()
	if mode != 0600 {
		t.Errorf("key file permissions: got %o, want %o", mode, 0600)
	}
}

func TestGetKeyTrims(t *testing.T) {
	tmpDir := setupTestVault(t)

	provider := "trim_test"
	providerDir := filepath.Join(tmpDir, provider)
	if err := os.MkdirAll(providerDir, 0700); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	// Записываем ключ с пробелами и переводом строки
	keyPath := filepath.Join(providerDir, "api.key")
	if err := os.WriteFile(keyPath, []byte("  key-with-spaces  \n"), 0600); err != nil {
		t.Fatalf("failed to write key: %v", err)
	}

	value, err := GetKey(provider, "api.key")
	if err != nil {
		t.Fatalf("GetKey failed: %v", err)
	}

	expected := "key-with-spaces"
	if value != expected {
		t.Errorf("expected %q, got %q", expected, value)
	}
}
