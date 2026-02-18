package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var allowed = map[string]map[string]bool{
	"cli": {
		"archive":   true,
		"discovery": true,
	},
	"discovery": {
		"model":    true,
		"runstore": true,
		"ytdlp":    true,
	},
	"archive": {
		"model":    true,
		"runstore": true,
		"ytdlp":    true,
	},
	"model":    {},
	"runstore": {},
	"ytdlp":    {},
}

func main() {
	violations := []string{}

	err := filepath.WalkDir("internal", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		srcPkg := sourcePackage(path)
		if srcPkg == "" {
			return nil
		}
		allowMap, ok := allowed[srcPkg]
		if !ok {
			violations = append(violations, fmt.Sprintf("%s: unknown source package %q", path, srcPkg))
			return nil
		}

		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}

		for _, imp := range file.Imports {
			impPath := strings.Trim(imp.Path.Value, "\"")
			tgtPkg, ok := targetPackage(impPath)
			if !ok {
				continue
			}
			if tgtPkg == srcPkg {
				continue
			}
			if !allowMap[tgtPkg] {
				violations = append(violations, fmt.Sprintf("%s: %s -> %s is forbidden", path, srcPkg, tgtPkg))
			}
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "boundary walk failed: %v\n", err)
		os.Exit(1)
	}

	if len(violations) > 0 {
		fmt.Fprintln(os.Stderr, "architecture boundary violations detected:")
		for _, v := range violations {
			fmt.Fprintf(os.Stderr, "- %s\n", v)
		}
		os.Exit(1)
	}

	fmt.Println("architecture boundary check: OK")
}

func sourcePackage(path string) string {
	parts := strings.Split(filepath.ToSlash(path), "/")
	if len(parts) < 2 || parts[0] != "internal" {
		return ""
	}
	return parts[1]
}

func targetPackage(importPath string) (string, bool) {
	const prefix = "yt-vod-manager/internal/"
	if !strings.HasPrefix(importPath, prefix) {
		return "", false
	}
	rest := strings.TrimPrefix(importPath, prefix)
	if rest == "" {
		return "", false
	}
	parts := strings.Split(rest, "/")
	return parts[0], true
}
