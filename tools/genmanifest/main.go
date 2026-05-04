// genmanifest scans a dist/ directory of cross-compiled orcha binaries,
// computes their sha256 sums, and writes a manifest.json suitable for upload
// alongside a GitHub release.
//
// Usage:
//
//	go run ./tools/genmanifest <dist-dir> <version> <repo-slug>
//
// Example:
//
//	go run ./tools/genmanifest dist 0.1.0 ryfoo/orcha > dist/manifest.json
//
// The manifest format is:
//
//	{
//	  "version": "0.1.0",
//	  "binaries": {
//	    "linux-amd64": {
//	      "url":    "https://github.com/.../orcha-linux-amd64",
//	      "sha256": "<hex>"
//	    },
//	    ...
//	  }
//	}
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: genmanifest <dist-dir> <version> <repo-slug>")
		os.Exit(2)
	}
	distDir := os.Args[1]
	version := os.Args[2]
	repo := os.Args[3] // e.g. "ryfoo/orcha"

	matches, err := filepath.Glob(filepath.Join(distDir, "orcha-*-*"))
	if err != nil {
		die(err)
	}
	if len(matches) == 0 {
		die(fmt.Errorf("no orcha-<os>-<arch> files in %s", distDir))
	}

	binaries := map[string]map[string]string{}
	for _, path := range matches {
		name := filepath.Base(path)
		platform := strings.TrimPrefix(name, "orcha-")
		platform = strings.TrimSuffix(platform, ".exe")

		hash, err := sha256File(path)
		if err != nil {
			die(err)
		}
		binaries[platform] = map[string]string{
			"url": fmt.Sprintf(
				"https://github.com/%s/releases/download/v%s/%s",
				repo, version, name,
			),
			"sha256": hash,
		}
	}

	out := map[string]any{
		"version":  version,
		"binaries": binaries,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		die(err)
	}
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "genmanifest:", err)
	os.Exit(1)
}
