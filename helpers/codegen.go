package helpers

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"

	"github.com/gofiber/fiber/v2/log"
)

func FileSubstitute(templatePath string, savePath string, values map[string]string) {
	createRecursive(filepath.Dir(savePath))
	templateContent, err := ioutil.ReadFile(templatePath)
	if err != nil {
		log.Infof("error reading template (%s): %v", templatePath, err)
		return
	}
	pattern := "port"
	reg, err := regexp.Compile(pattern)

	saveContent := reg.ReplaceAllString(string(templateContent), "Hello")
	err = ioutil.WriteFile(savePath, []byte(saveContent), 0644)
	if err != nil {
		log.Infof("error saving new content to %s: %v", savePath, err)
		return
	}
}

func createRecursive(saveDir string) {
	err := os.MkdirAll(saveDir, 0755)
	if err != nil {
		log.Infof("failed to create directory (%s): %v", saveDir, err)
	}
}
