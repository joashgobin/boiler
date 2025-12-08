package helpers

import (
	"golang.org/x/image/draw"
	"image"

	"fmt"
	"github.com/Kagami/go-avif"
	"github.com/gofiber/fiber/v2/log"
	"github.com/kolesa-team/go-webp/encoder"
	"github.com/kolesa-team/go-webp/webp"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type SafeImage struct {
	mu    sync.Mutex
	image image.Image
	from  string
	to    string
}

func NewSafeImage(img image.Image, from, to string) *SafeImage {
	return &SafeImage{image: img, from: from, to: to}
}

func (si *SafeImage) SaveAVIF() string {
	si.mu.Lock()
	defer si.mu.Unlock()

	output, err := os.Create(si.from)
	if err != nil {
		log.Errorf("error creating output path: %v", err)
		return ""
	}
	defer output.Close()

	if err := avif.Encode(output, si.image, nil); err != nil {
		log.Errorf("error encoding safe image to avif: %v", err)
		return ""
	}

	err = os.Rename(si.from, si.to)
	if err != nil {
		log.Errorf("error renaming safe image for avif: %v", err)
		return ""
	}
	return si.to
}

func (si *SafeImage) SaveWebp() string {
	si.mu.Lock()
	defer si.mu.Unlock()

	output, err := os.Create(si.from)
	if err != nil {
		log.Errorf("error creating output path: %v", err)
		return ""
	}
	defer output.Close()
	options, err := encoder.NewLossyEncoderOptions(encoder.PresetDefault, 75)
	if err != nil {
		log.Errorf("error setting safe image webp options: %v", err)
		return ""
	}
	if err := webp.Encode(output, si.image, options); err != nil {
		log.Errorf("error encoding safe image to webp: %v", err)
		return ""
	}

	err = os.Rename(si.from, si.to)
	if err != nil {
		log.Errorf("error renaming safe image for webp: %v", err)
		return ""
	}
	return si.to
}

func ConvertInlineAVIF(srcPath string, toDir string, dimensions ...int) string {
	width := 600
	if len(dimensions) > 0 {
		width = dimensions[0]
	}
	fromDir := filepath.Dir(srcPath)
	start := time.Now()

	hashString := GetFileHash(srcPath)

	tempPath := fmt.Sprintf("%s_%dx_%v.%s.avif.%d.lock",
		strings.TrimSuffix(strings.Replace(srcPath, fromDir, toDir, -1),
			filepath.Ext(srcPath)), width, time.Now().Format(time.RFC3339), hashString, os.Getpid())
	outputPath := fmt.Sprintf("%s_%dx.%s.avif",
		strings.TrimSuffix(strings.Replace(srcPath, fromDir, toDir, -1),
			filepath.Ext(srcPath)), width, hashString)

	if FileExists(outputPath) {
		// log.Info("skipping ", outputPath)
		return outputPath
	}
	file, err := os.Open(srcPath)
	if err != nil {
		log.Errorf("error converting to avif: %v", err)
		return ""
	}

	var img image.Image

	switch filepath.Ext(srcPath) {
	case ".png":
		img, err = png.Decode(file)
		if err != nil {
			log.Errorf("error converting to avif: %v", err)
			return ""
		}
	case ".jpg", ".jpeg":
		img, err = jpeg.Decode(file)
		if err != nil {
			log.Errorf("error converting to avif: %v", err)
			return ""
		}
	}

	// resizing attempt on final image
	ratio := (float64)(img.Bounds().Max.Y) / (float64)(img.Bounds().Max.X)
	height := int(math.Round(float64(width) * ratio))

	// create final image with new size
	finalImg := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(finalImg, finalImg.Rect, img, img.Bounds(), draw.Over, nil)

	safeImage := NewSafeImage(finalImg, tempPath, outputPath)
	safeImage.SaveAVIF()

	log.Infof("(%v) converted image (%s) to avif: %s", time.Since(start), srcPath, outputPath)
	return outputPath
}

func ConvertInlineWebp(srcPath string, toDir string, dimensions ...int) string {
	width := 600
	if len(dimensions) > 0 {
		width = dimensions[0]
	}
	fromDir := filepath.Dir(srcPath)
	start := time.Now()

	hashString := GetFileHash(srcPath)

	tempPath := fmt.Sprintf("%s_%dx_%v.%s.webp.%d.lock",
		strings.TrimSuffix(strings.Replace(srcPath, fromDir, toDir, -1),
			filepath.Ext(srcPath)), width, time.Now().Format(time.RFC3339), hashString, os.Getpid())
	outputPath := fmt.Sprintf("%s_%dx.%s.webp",
		strings.TrimSuffix(strings.Replace(srcPath, fromDir, toDir, -1),
			filepath.Ext(srcPath)), width, hashString)

	if FileExists(outputPath) {
		// log.Info("skipping ", outputPath)
		return outputPath
	}
	file, err := os.Open(srcPath)
	if err != nil {
		log.Errorf("error converting to webp: %v", err)
		return ""
	}

	var img image.Image

	switch filepath.Ext(srcPath) {
	case ".png":
		img, err = png.Decode(file)
		if err != nil {
			log.Errorf("error converting to webp: %v", err)
			return ""
		}
	case ".jpg", ".jpeg":
		img, err = jpeg.Decode(file)
		if err != nil {
			log.Errorf("error converting to webp: %v", err)
			return ""
		}
	}

	// resizing attempt on final image
	ratio := (float64)(img.Bounds().Max.Y) / (float64)(img.Bounds().Max.X)
	height := int(math.Round(float64(width) * ratio))

	// create final image with new size
	finalImg := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(finalImg, finalImg.Rect, img, img.Bounds(), draw.Over, nil)

	safeImage := NewSafeImage(finalImg, tempPath, outputPath)
	safeImage.SaveWebp()

	log.Infof("(%v) converted image (%s) to webp: %s", time.Since(start), srcPath, outputPath)
	return outputPath
}

func ConvertToAVIF(srcPath string, fileListPtr *map[string]string, fromDir, toDir string) error {
	start := time.Now()
	hashString := GetFileHash(srcPath)
	outputPath := fmt.Sprintf("%s.%s.avif",
		strings.TrimSuffix(strings.Replace(srcPath, fromDir, toDir, -1),
			filepath.Ext(srcPath)), hashString)

	if FileExists(outputPath) {
		// log.Info("skipping ", outputPath)
		if fileListPtr != nil {
			(*fileListPtr)[strings.TrimPrefix(srcPath, "static/")] = outputPath
		}
		return nil
	}
	file, err := os.Open(srcPath)
	if err != nil {
		return err
	}

	var img image.Image

	switch filepath.Ext(srcPath) {
	case ".png":
		img, err = png.Decode(file)
		if err != nil {
			return err
		}
	case ".jpg", ".jpeg":
		img, err = jpeg.Decode(file)
		if err != nil {
			return err
		}
	}

	output, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer output.Close()
	if err := avif.Encode(output, img, nil); err != nil {
		return err
	}
	log.Infof("(%v) converted image (%s) to avif: %s", time.Since(start), srcPath, outputPath)
	if fileListPtr != nil {
		(*fileListPtr)[strings.TrimPrefix(srcPath, "static/")] = outputPath
	}
	return nil
}

func ConvertToWebp(srcPath string, fileListPtr *map[string]string, fromDir, toDir string) error {
	start := time.Now()
	hashString := GetFileHash(srcPath)
	outputPath := fmt.Sprintf("%s.%s.webp",
		strings.TrimSuffix(strings.Replace(srcPath, fromDir, toDir, -1),
			filepath.Ext(srcPath)), hashString)

	if FileExists(outputPath) {
		// log.Info("skipping ", outputPath)
		if fileListPtr != nil {
			(*fileListPtr)[strings.TrimPrefix(srcPath, "static/")] = outputPath
		}
		return nil
	}
	file, err := os.Open(srcPath)
	if err != nil {
		return err
	}

	var img image.Image

	switch filepath.Ext(srcPath) {
	case ".png":
		img, err = png.Decode(file)
		if err != nil {
			return err
		}
	case ".jpg", ".jpeg":
		img, err = jpeg.Decode(file)
		if err != nil {
			return err
		}
	}

	output, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer output.Close()
	options, err := encoder.NewLossyEncoderOptions(encoder.PresetDefault, 75)
	if err != nil {
		return err
	}
	if err := webp.Encode(output, img, options); err != nil {
		return err
	}
	log.Infof("(%v) converted image (%s) to webp: %s", time.Since(start), srcPath, outputPath)
	if fileListPtr != nil {
		(*fileListPtr)[strings.TrimPrefix(srcPath, "static/")] = outputPath
	}
	return nil
}

func ConvertInFolderToAVIF(folderPath string, targetFolder string, ext string, fileListPtr *map[string]string) {
	err := os.MkdirAll(targetFolder, 0755)
	if err != nil {
		log.Infof("failed to create directory %s", targetFolder)
	}
	entries, err := os.ReadDir(folderPath)
	if err != nil {
		fmt.Printf("error reading directory (%s): %v\n", folderPath, err)
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ext {
			err := ConvertToAVIF(filepath.Join(folderPath, entry.Name()), fileListPtr, folderPath, targetFolder)
			if err != nil {
				log.Errorf("could not convert file (%s) to avif: err\n", entry.Name(), err)
			}
		}
	}

}

func ConvertInFolderToWebp(folderPath string, targetFolder string, ext string, fileListPtr *map[string]string) {
	err := os.MkdirAll(targetFolder, 0755)
	if err != nil {
		log.Infof("failed to create directory %s", targetFolder)
	}
	entries, err := os.ReadDir(folderPath)
	if err != nil {
		fmt.Printf("error reading directory (%s): %v\n", folderPath, err)
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ext {
			err := ConvertToWebp(filepath.Join(folderPath, entry.Name()), fileListPtr, folderPath, targetFolder)
			if err != nil {
				log.Errorf("could not convert file (%s) to webp: err\n", entry.Name(), err)
			}
		}
	}

}
