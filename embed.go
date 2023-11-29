package frankenphp

import (
	"crypto/md5"
	"embed"
	_ "embed"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const embedDir = "embed"

// The path of the embedded PHP application (empty if none)
var EmbeddedAppPath string

//go:embed all:embed
var embeddedApp embed.FS

func init() {
	entries, err := embeddedApp.ReadDir(embedDir)
	if err != nil {
		panic(err)
	}

	if len(entries) == 1 && entries[0].Name() == ".gitignore" {
		//no embedded app
		return
	}

	e, err := os.Executable()
	if err != nil {
		panic(err)
	}

	e, err = filepath.EvalSymlinks(e)
	if err != nil {
		panic(err)
	}

	// TODO: use XXH3 instead of MD5
	h := md5.Sum([]byte(e))
	appPath := filepath.Join(os.TempDir(), "frankenphp_"+hex.EncodeToString(h[:]))

	if err := os.RemoveAll(appPath); err != nil {
		panic(err)
	}
	if err := copyToDisk(appPath, embedDir, entries); err != nil {
		os.RemoveAll(appPath)
		panic(err)
	}

	EmbeddedAppPath = appPath
}

func copyToDisk(appPath string, currentDir string, entries []fs.DirEntry) error {
	if err := os.Mkdir(appPath+strings.TrimPrefix(currentDir, embedDir), 0700); err != nil {
		return err
	}

	for _, entry := range entries {
		name := entry.Name()

		if entry.IsDir() {
			entries, err := embeddedApp.ReadDir(currentDir + "/" + name)
			if err != nil {
				return err
			}

			if err := copyToDisk(appPath, currentDir+"/"+name, entries); err != nil {
				return err
			}

			continue
		}

		data, err := embeddedApp.ReadFile(currentDir + "/" + name)
		if err != nil {
			return err
		}

		f := appPath + "/" + strings.TrimPrefix(currentDir, embedDir) + "/" + name
		if err := os.WriteFile(f, data, 0500); err != nil {
			return err
		}
	}

	return nil
}
