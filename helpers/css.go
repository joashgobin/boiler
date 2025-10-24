package helpers

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

func GetEmbedFiles(fs *embed.FS, path string) ([]string, error) {
	entries, err := fs.ReadDir(filepath.Clean(path))
	if err != nil {
		return nil, err
	}

	var out []string
	for _, entry := range entries {
		fullPath := filepath.Join(path, entry.Name())
		if entry.IsDir() {
			res, err := GetEmbedFiles(fs, fullPath)
			if err != nil {
				return nil, err
			}
			out = append(out, res...)
			continue
		}
		out = append(out, fullPath)
	}

	return out, nil
}

func ExtractClassNames(filePath string, classes *[]string) error {
	// Read file content
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %v", err)
	}

	// HTML class attribute regex pattern
	re := regexp.MustCompile(`class="([^"]+)"|class='([^']+)'`)

	// Find all matches
	matches := re.FindAllStringSubmatch(string(data), -1)

	// Process matches and split by spaces
	for _, match := range matches {
		// Handle both double quotes and single quotes
		classList := match[1]
		if classList == "" {
			classList = match[2]
		}

		// Split by spaces and add individual classes
		for _, className := range strings.Split(classList, " ") {
			if className != "" { // Skip empty entries
				if !slices.Contains(*classes, className) {
					*classes = append(*classes, className)
				}
			}
		}
	}

	return nil
}

func SaveCSSClasses(fs *embed.FS, targetFile string, cssFiles ...string) error {
	viewFiles, err := GetEmbedFiles(fs, "views")
	classes := []string{}
	if err != nil {
		return err
	}
	for _, file := range viewFiles {
		// fmt.Println(file)
		err = ExtractClassNames(file, &classes)
		if err != nil {
			return err
		}
	}
	fmt.Println(classes)
	for _, file := range cssFiles {
		fmt.Println("optimizing:", file)
		data, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read file: %v", err)
		}

		targetCSSstring := string(data)
		// fmt.Println(targetCSSstring)

		// getting chunks that match the regex \n*\.([^{]*){[^}]*}|@[^{]*{([^{]*){[^}]*}[^}*]}
		// classExp := `(?m)^\n*?([^{]*?){[^}]*?}`
		queryExp := `(?m)@([^{]*?{[^{]*?){([^}]*?)*}([^}])*}`

		// classRe := regexp.MustCompile(classExp)
		queryRe := regexp.MustCompile(queryExp)

		// classMatches := classRe.FindAllStringSubmatch(targetCSSstring, -1)
		queryMatches := queryRe.FindAllStringSubmatch(targetCSSstring, -1)

		/*for _, match := range classMatches {
			fmt.Printf("Selector (CSS): %s\n", match[1])
			// fmt.Printf("Class match: %s\n", match[0])
		}*/
		for _, match := range queryMatches {
			fmt.Printf("Query (CSS): %s\n", match[1])
			// fmt.Printf("Query match: %s\n", match[0])
		}
	}
	return nil
}
