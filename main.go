package main

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type ChangedFiles struct {
	Added    []string
	Modified []string
	Removed  []string
}

type FileIndex struct {
	Index map[string]string
}

func NewIndexInformation() *FileIndex {
	return &FileIndex{
		Index: make(map[string]string),
	}
}

func NewChangedFiles() *ChangedFiles {
	return &ChangedFiles{
		Added:    make([]string, 0),
		Modified: make([]string, 0),
		Removed:  make([]string, 0),
	}
}

func (cf *ChangedFiles) IsEmpty() bool {
	return !cf.HasChanges()
}

func (cf *ChangedFiles) HasChanges() bool {
	return len(cf.Added) > 0 || len(cf.Modified) > 0 || len(cf.Removed) > 0
}

func (fi *FileIndex) Add(path, hash string) {
	fi.Index[path] = hash
}

func (fi *FileIndex) Size() int {
	return len(fi.Index)
}

func (fi *FileIndex) Contains(path string) bool {
	_, ok := fi.Index[path]
	return ok
}

func (fi *FileIndex) Filename(path string) string {
	return filepath.Join(path, ".project-indexer.idx")
}

func (fi *FileIndex) WriteIndexFile(path string) error {
	indexFilePath := fi.Filename(path)
	f, err := os.Create(indexFilePath)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	err = encoder.Encode(fi.Index)

	return err
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: project-indexer [index|check] <directory>")
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "index":
		if len(os.Args) != 3 {
			fmt.Println("Usage: project-indexer index <directory>")
			os.Exit(1)
		}
		directory := os.Args[2]
		_, err := indexDirectory(directory)
		if err != nil {
			fmt.Println("Error indexing directory:", err)
			os.Exit(1)
		}
	case "check":
		if len(os.Args) != 3 {
			fmt.Println("Usage: project-indexer check <directory>")
			os.Exit(1)
		}
		directory := os.Args[2]
		_, err := checkDirectory(directory)
		if err != nil {
			fmt.Println("Error checking directory:", err)
			os.Exit(1)
		}
	case "has-changes":
		if len(os.Args) != 3 {
			fmt.Println("Usage: project-indexer has-changes <directory>")
			os.Exit(1)
		}
		directory := os.Args[2]
		hasChanges, err := hasChanges(directory)

		if err != nil {
			fmt.Println("Error checking directory:", err)
			os.Exit(1)
		}

		if hasChanges {
			os.Exit(1)
		}

		os.Exit(0)
	default:
		fmt.Println("Unknown command:", cmd)
		fmt.Println("Usage: project-indexer [index|check] <directory>")
		os.Exit(1)
	}
}

func hasChanges(dir string) (bool, error) {
	changes, err := checkDirectory(dir)
	if err != nil {
		return false, err
	}
	return changes.HasChanges(), nil
}

func indexDirectory(dir string) (*FileIndex, error) {
	result := NewIndexInformation()
	index := make(map[string]string)
	counter := 0

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			relPath, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}

			if relPath == ".project-indexer.idx" {
				return nil // Skip the index file itself
			}

			if strings.HasSuffix(relPath, ".git") {
				return nil
			}

			// ignore test files
			matched, err := regexp.MatchString(`\.test\.(tsx|ts|js|jsx|json)$`, relPath)
			if matched || err != nil {
				return nil
			}

			// only hash certain file types
			matched, err = regexp.MatchString(`.+\.(ts|js|tsx|jsx|mjs|cjs|mts|json|css|sass|scss|png|svg|jpg|gql)$`, relPath)
			if !matched || err != nil {
				return nil
			}

			hash, err := computeFileHash(path)
			if err != nil {
				return err
			}

			counter++
			result.Add(relPath, hash)
			index[relPath] = hash
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	result.WriteIndexFile(dir)

	// indexFilePath := filepath.Join(dir, ".project-indexer.idx")
	// f, err := os.Create(indexFilePath)
	// if err != nil {
	// 	return nil, err
	// }
	// defer f.Close()

	// encoder := json.NewEncoder(f)
	// err = encoder.Encode(index)
	// if err != nil {
	// 	return nil, err
	// }

	fmt.Printf("Indexed %d files\n", result.Size())
	fmt.Println("Index created at", result.Filename(dir))

	return result, nil
}

func checkDirectory(dir string) (*ChangedFiles, error) {
	result := NewChangedFiles()
	indexFilePath := filepath.Join(dir, ".project-indexer.idx")
	f, err := os.Open(indexFilePath)
	if err != nil {
		return nil, fmt.Errorf("could not open index file: %v", err)
	}
	defer f.Close()

	var storedIndex map[string]string
	decoder := json.NewDecoder(f)
	err = decoder.Decode(&storedIndex)
	if err != nil {
		return nil, fmt.Errorf("could not decode index file: %v", err)
	}

	currentIndex := make(map[string]string)

	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			relPath, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}

			if relPath == ".project-indexer.idx" {
				return nil // Skip the index file itself
			}

			if strings.HasSuffix(relPath, ".git") {
				return nil
			}

			// ignore test files
			matched, err := regexp.MatchString(`\.test\.(tsx|ts|js|jsx|json)$`, relPath)
			if matched || err != nil {
				return nil
			}

			// only hash certain file types
			matched, err = regexp.MatchString(`.+\.(ts|js|tsx|jsx|mjs|cjs|mts|json|css|sass|scss|png|svg|jpg|gql)$`, relPath)
			if !matched || err != nil {
				return nil
			}

			hash, err := computeFileHash(path)
			if err != nil {
				return err
			}

			currentIndex[relPath] = hash
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Check for added or modified files
	for path, currentHash := range currentIndex {
		if storedHash, ok := storedIndex[path]; ok {
			if currentHash != storedHash {
				result.Modified = append(result.Modified, path)
			}
		} else {
			result.Added = append(result.Added, path)
		}
	}

	// Check for removed files
	for path := range storedIndex {
		if _, ok := currentIndex[path]; !ok {
			result.Removed = append(result.Removed, path)
		}
	}

	if result.IsEmpty() {
		fmt.Println("No changes detected.")
	}

	if result.HasChanges() {
		if len(result.Added) > 0 {
			fmt.Println("Added files:")
			for _, f := range result.Added {
				fmt.Println("  ", f)
			}
		}
		if len(result.Modified) > 0 {
			fmt.Println("Modified files:")
			for _, f := range result.Modified {
				fmt.Println("  ", f)
			}
		}
		if len(result.Removed) > 0 {
			fmt.Println("Removed files:")
			for _, f := range result.Removed {
				fmt.Println("  ", f)
			}
		}
	}

	return result, nil
}

func computeFileHash(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
