package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrPathRequired     = errors.New("workspace path is required")
	ErrOutsideWorkspace = errors.New("path is outside workspace")
)

type Boundary struct {
	Root string
}

func NewBoundary(root string) (Boundary, error) {
	var err error
	if strings.TrimSpace(root) == "" {
		root, err = DefaultPath()
	} else {
		root, err = filepath.Abs(root)
	}
	if err != nil {
		return Boundary{}, err
	}

	return Boundary{Root: filepath.Clean(root)}, nil
}

func (b Boundary) Resolve(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", ErrPathRequired
	}

	target := path
	if !filepath.IsAbs(target) {
		target = filepath.Join(b.Root, target)
	}

	absTarget, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	absTarget = filepath.Clean(absTarget)
	if !b.Contains(absTarget) {
		return "", ErrOutsideWorkspace
	}
	return absTarget, nil
}

func (b Boundary) Contains(path string) bool {
	if strings.TrimSpace(b.Root) == "" || strings.TrimSpace(path) == "" {
		return false
	}

	root, err := filepath.Abs(b.Root)
	if err != nil {
		return false
	}
	target, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(target))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && !filepath.IsAbs(rel))
}

func (b Boundary) Relative(path string) (string, error) {
	if !b.Contains(path) {
		return "", ErrOutsideWorkspace
	}

	rel, err := filepath.Rel(b.Root, path)
	if err != nil {
		return "", err
	}
	if rel == "." {
		return ".", nil
	}
	return filepath.ToSlash(rel), nil
}
