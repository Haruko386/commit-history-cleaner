package main

import (
	"os"
	"path/filepath"
)

// desktopCacheDir resolves the default desktop cache directory.
func desktopCacheDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	appDir := filepath.Join(configDir, "FunPDF")
	cacheDir := filepath.Join(appDir, "cache")

	err = os.MkdirAll(cacheDir, 0755)
	if err != nil {
		return "", err
	}
	return cacheDir, nil
}

// desktopDatabasePath resolves the desktop SQLite database path.
func desktopDatabasePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	addDir := filepath.Join(configDir, "FunPDF")

	err = os.MkdirAll(addDir, 0755)
	if err != nil {
		return "", err
	}

	return filepath.Join(addDir, "FunPDF.db"), nil
}
