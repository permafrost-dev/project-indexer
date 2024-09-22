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

	flags "github.com/jessevdk/go-flags"
)

var opts struct {
	Help     bool   `short:"h" long:"help" description:"Show this help message"`
	Filename string `short:"f" long:"filename" description:"Index filename to read and write"`

	Ignored []string `short:"i" long:"ignore" description:"Ignore files matching the given pattern" required:"no"`

	Args struct {
		Paths []string
	} `positional-args:"yes" required:"yes"`
}

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
	indexFilePath := path //fi.Filename(path)
	if !strings.HasSuffix(indexFilePath, ".idx") {
		indexFilePath = filepath.Join(path, ".project-indexer.idx")
	}
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
		fmt.Printf("Usage: %s %s <path1, path2, ...> [-f <filename>]\n", os.Args[0], "[index|check]")
		os.Exit(1)
	}

	cmd := os.Args[1]

	if len(os.Args) < 3 {
		if len(os.Args) < 2 {
			cmd = "<command>"
		}

		fmt.Printf("Usage: %s %s <path1, path2, ...> [-f <filename>]\n", os.Args[0], cmd)
		os.Exit(1)
	}

	flags.ParseArgs(&opts, os.Args[2:])

	if len(opts.Filename) == 0 {
		opts.Filename = ".project-indexer.idx"
	}

	switch cmd {
	case "index":
		index := NewIndexInformation()
		hasError := false
		for _, directory := range opts.Args.Paths {
			idx, err := indexDirectory(directory)
			if err != nil {
				fmt.Println("Error indexing directory:", err)
				hasError = true
			}

			for fn, hash := range idx.Index {
				index.Add(fn, hash)
			}
		}

		index.WriteIndexFile(opts.Filename)

		fmt.Printf("Indexed %d files\n", index.Size())
		fmt.Printf("Index written to %s\n", opts.Filename)

		if hasError {
			os.Exit(1)
		}

		os.Exit(0)
	case "check":
		if len(os.Args) < 3 {
			fmt.Println("Usage: project-indexer check <directory>")
			os.Exit(1)
		}
		result := NewChangedFiles()

		currentFullIndex := make(map[string]string)
		storedFullIndex := make(map[string]string)

		for _, path := range opts.Args.Paths {
			currentIndex, storedIndex, err := checkDirectory(opts.Filename, path)
			if err != nil {
				fmt.Println("Error checking directory:", err)
				os.Exit(1)
			}

			for path, currentHash := range currentIndex {
				currentFullIndex[path] = currentHash
			}

			storedFullIndex = storedIndex
		}

		for path, currentHash := range currentFullIndex {
			if storedHash, ok := storedFullIndex[path]; ok {
				if currentHash != storedHash {
					result.Modified = append(result.Modified, path)
				}
			} else {
				result.Added = append(result.Added, path)
			}
		}

		for path, _ := range storedFullIndex {
			if _, ok := currentFullIndex[path]; !ok {
				result.Removed = append(result.Removed, path)
			}
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

			os.Exit(1)
		}

		fmt.Println("No changes detected.")
		os.Exit(0)
	default:
		fmt.Println("Unknown command:", cmd)
		fmt.Println("Usage: project-indexer [index|check] <directory>")
		os.Exit(1)
	}
}

func FindProjectRoot(dir string) string {
	originalDir := dir
	//find the root of the project by locating the dir with a .git/node_modules folder:
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		if _, err := os.Stat(filepath.Join(dir, "node_modules")); err == nil {
			return dir
		}
		if dir == "/" {
			result := filepath.Dir(originalDir)
			return result
		}
		dir = filepath.Dir(dir)
	}
}

func indexDirectory(dir string) (*FileIndex, error) {
	result := NewIndexInformation()
	projectDir := FindProjectRoot(dir)

	index := make(map[string]string)
	counter := 0

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			relPath, err := filepath.Rel(projectDir, path)
			if err != nil {
				return err
			}

			if relPath == ".project-indexer.idx" || strings.HasSuffix(relPath, ".idx") {
				return nil // Skip the index file itself
			}

			if strings.HasSuffix(dir, ".git") {
				return nil
			}

			// ignore test files
			matched, err := regexp.MatchString(`\.test\.(tsx|ts|js|jsx|json)$`, relPath)
			if matched || err != nil {
				return nil
			}

			// only hash certain file types
			matched, err = regexp.MatchString(`.+\.(php|ts|js|tsx|jsx|mjs|cjs|mts|json|css|sass|scss|png|svg|jpg|gql)$`, relPath)
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

	// result.WriteIndexFile(dir)

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

	// fmt.Printf("Indexed %d files\n", result.Size())
	// fmt.Println("Index created at", result.Filename(dir))

	return result, nil
}

func checkDirectory(indexFilePath string, dir string) (map[string]string, map[string]string, error) {
	//result := NewChangedFiles()
	projectDir := FindProjectRoot(dir)
	f, err := os.Open(indexFilePath)
	if err != nil {
		return nil, nil, fmt.Errorf("could not open index file: %v", err)
	}
	defer f.Close()

	var storedIndex map[string]string
	decoder := json.NewDecoder(f)
	err = decoder.Decode(&storedIndex)
	if err != nil {
		return nil, nil, fmt.Errorf("could not decode index file: %v", err)
	}

	currentIndex := make(map[string]string)

	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			relPath, err := filepath.Rel(projectDir, path)
			if err != nil {
				return err
			}

			if relPath == ".project-indexer.idx" || strings.HasSuffix(relPath, ".idx") {
				return nil // Skip the index file itself
			}

			if strings.HasSuffix(dir, ".git") {
				return nil
			}

			// ignore test files
			matched, err := regexp.MatchString(`\.test\.(tsx|ts|js|jsx|json)$`, relPath)
			if matched || err != nil {
				return nil
			}

			// only hash certain file types
			matched, err = regexp.MatchString(`.+\.(php|ts|js|tsx|jsx|mjs|cjs|mts|json|css|sass|scss|png|svg|jpg|gql)$`, relPath)
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
		return nil, nil, err
	}

	return currentIndex, storedIndex, nil

	// fmt.Printf("stored index: %v\n", storedIndex)
	// Check for added or modified files
	// for path, currentHash := range currentIndex {
	// 	if storedHash, ok := storedIndex[path]; ok {
	// 		if currentHash != storedHash {
	// 			result.Modified = append(result.Modified, path)
	// 		}
	// 	} else {
	// 		result.Added = append(result.Added, path)
	// 	}
	// }

	// // check for items in storedIndex that are not in currentIndex

	// for path, _ := range storedIndex {
	// 	if _, ok := currentIndex[path]; !ok {
	// 		result.Removed = append(result.Removed, path)
	// 	}
	// }

	// Check for removed files
	// for path, _ := range storedIndex {
	// 	if currentPath, ok := currentIndex[path]; !ok {
	// 		fmt.Printf("stored/current path: %v, %v\n", path, currentPath)
	// 		result.Removed = append(result.Removed, path)
	// 	}
	// }

	// if result.IsEmpty() {
	// 	fmt.Println("No changes detected.")
	// }

	// if result.HasChanges() {
	// 	if len(result.Added) > 0 {
	// 		fmt.Println("Added files:")
	// 		for _, f := range result.Added {
	// 			fmt.Println("  ", f)
	// 		}
	// 	}
	// 	if len(result.Modified) > 0 {
	// 		fmt.Println("Modified files:")
	// 		for _, f := range result.Modified {
	// 			fmt.Println("  ", f)
	// 		}
	// 	}
	// 	if len(result.Removed) > 0 {
	// 		fmt.Println("Removed files:")
	// 		for _, f := range result.Removed {
	// 			fmt.Println("  ", f)
	// 		}
	// 	}
	// }
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
