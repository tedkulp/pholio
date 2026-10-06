package config

// VaultKeyNames lists every Vault key, for tests that check the init config
// covers them all.
func VaultKeyNames() []string {
	names := make([]string, 0, len(vaultKeys))
	for k := range vaultKeys {
		names = append(names, k)
	}
	return names
}
