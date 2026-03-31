package navigator

import (
	"path/filepath"
	"strings"
)

// AlternateFile returns the alternate file path for the given file.
// For Go: foo.go <-> foo_test.go
// For JS/TS: Component.tsx <-> Component.test.tsx
// For Python: module.py <-> test_module.py
// Returns empty string if no alternate pattern matches or the alternate doesn't exist.
func AlternateFile(filePath string) string {
	dir := filepath.Dir(filePath)
	base := filepath.Base(filePath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	var candidates []string

	switch ext {
	case ".go":
		if strings.HasSuffix(name, "_test") {
			// test -> impl
			impl := strings.TrimSuffix(name, "_test") + ext
			candidates = append(candidates, filepath.Join(dir, impl))
		} else {
			// impl -> test
			test := name + "_test" + ext
			candidates = append(candidates, filepath.Join(dir, test))
		}

	case ".ts", ".tsx", ".js", ".jsx":
		// Check various test naming patterns
		if strings.HasSuffix(name, ".test") {
			// Component.test.tsx -> Component.tsx
			impl := strings.TrimSuffix(name, ".test") + ext
			candidates = append(candidates, filepath.Join(dir, impl))
		} else if strings.HasSuffix(name, ".spec") {
			// Component.spec.tsx -> Component.tsx
			impl := strings.TrimSuffix(name, ".spec") + ext
			candidates = append(candidates, filepath.Join(dir, impl))
		} else {
			// Component.tsx -> Component.test.tsx, Component.spec.tsx
			candidates = append(candidates,
				filepath.Join(dir, name+".test"+ext),
				filepath.Join(dir, name+".spec"+ext),
			)
			// Also check __tests__ directory
			candidates = append(candidates,
				filepath.Join(dir, "__tests__", base),
				filepath.Join(dir, "__tests__", name+".test"+ext),
			)
		}

	case ".py":
		if strings.HasPrefix(name, "test_") {
			// test_module.py -> module.py
			impl := strings.TrimPrefix(name, "test_") + ext
			candidates = append(candidates, filepath.Join(dir, impl))
		} else if strings.HasSuffix(name, "_test") {
			// module_test.py -> module.py
			impl := strings.TrimSuffix(name, "_test") + ext
			candidates = append(candidates, filepath.Join(dir, impl))
		} else {
			// module.py -> test_module.py, module_test.py
			candidates = append(candidates,
				filepath.Join(dir, "test_"+name+ext),
				filepath.Join(dir, name+"_test"+ext),
			)
			// Also check tests/ directory
			candidates = append(candidates,
				filepath.Join(dir, "tests", "test_"+name+ext),
			)
		}
	}

	// Return first existing candidate
	for _, c := range candidates {
		if fileExists(c) {
			return c
		}
	}

	// Return first candidate even if it doesn't exist (for creating)
	if len(candidates) > 0 {
		return candidates[0]
	}
	return ""
}

func fileExists(path string) bool {
	_, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	// Use Lstat to avoid following symlinks
	info, err := filepath.Glob(path)
	return err == nil && len(info) > 0
}
