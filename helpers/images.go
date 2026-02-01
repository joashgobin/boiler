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
	mu         sync.Mutex
	diskImage  image.Image
	finalImage image.RGBA
}

func (si *SafeImage) SaveAVIF(from, to string) string {
	output, err := os.Create(from)
	if err != nil {
		log.Errorf("error creating output path: %v", err)
		return ""
	}
	defer output.Close()

	if err := avif.Encode(output, &si.finalImage, nil); err != nil {
		log.Errorf("error encoding safe image to avif: %v", err)
		return ""
	}

	err = os.Rename(from, to)
	if err != nil {
		log.Errorf("error renaming safe image for avif: %v", err)
		return ""
	}
	return to
}

func (si *SafeImage) SaveJPEG(from, to string) string {
	output, err := os.Create(from)
	if err != nil {
		log.Errorf("error creating output path: %v", err)
		return ""
	}
	defer output.Close()

	err = jpeg.Encode(output, &si.finalImage, &jpeg.Options{Quality: 90})
	if err != nil {
		log.Errorf("error encoding safe image to jpeg: %v", err)
		return ""
	}

	err = os.Rename(from, to)
	if err != nil {
		log.Errorf("error renaming safe image for jpeg: %v", err)
		return ""
	}
	return to
}

func (si *SafeImage) SavePNG(from, to string) string {
	output, err := os.Create(from)
	if err != nil {
		log.Errorf("error creating output path: %v", err)
		return ""
	}
	defer output.Close()

	err = png.Encode(output, &si.finalImage)
	if err != nil {
		log.Errorf("error encoding safe image to png: %v", err)
		return ""
	}

	err = os.Rename(from, to)
	if err != nil {
		log.Errorf("error renaming safe image for png: %v", err)
		return ""
	}
	return to
}

func (si *SafeImage) SaveWebp(from, to string) string {
	output, err := os.Create(from)
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
	if err := webp.Encode(output, &si.finalImage, options); err != nil {
		log.Errorf("error encoding safe image to webp: %v", err)
		return ""
	}

	err = os.Rename(from, to)
	if err != nil {
		log.Errorf("error renaming safe image for webp: %v", err)
		return ""
	}
	return to
}

func ConvertInlineAVIF(srcPath string, toDir string, dimensions ...int) string {
	width := 600
	intermediateWidth := 1200

	if len(dimensions) > 0 {
		width = dimensions[0]
	}
	fromDir := filepath.Dir(srcPath)
	start := time.Now()
	hashString := GetFileHash(srcPath)

	outputPath := fmt.Sprintf("%s_%dx.%s.avif",
		strings.TrimSuffix(strings.Replace(srcPath, fromDir, toDir, -1),
			filepath.Ext(srcPath)), width, hashString)

	if !FileExists(outputPath) {
		intermediatePath := fmt.Sprintf("%s_%dx.%s%s",
			strings.TrimSuffix(strings.Replace(srcPath, fromDir, toDir, -1),
				filepath.Ext(srcPath)), intermediateWidth, hashString, filepath.Ext(srcPath))

		// use intermediate if present
		if !FileExists(intermediatePath) {
			var si SafeImage
			si.SaveImage(srcPath, intermediatePath, intermediateWidth)
		}

		if FileExists(outputPath) {
			// log.Info("skipping ", outputPath)
			return outputPath
		}

		var si SafeImage
		si.SaveImage(intermediatePath, outputPath, width)

		log.Infof("(%v) converted image (%s) to webp: %s", time.Since(start), srcPath, outputPath)
	}
	return outputPath
}

func GetTempName(name string) string {
	return fmt.Sprintf("%s.%s.%d.lock", name, time.Now().Format(time.RFC3339), os.Getpid())
}

func (si *SafeImage) SaveImage(fromPath, toPath string, width int) {
	si.mu.Lock()
	defer si.mu.Unlock()

	tempPath := GetTempName(toPath)
	toExt := filepath.Ext(toPath)

	log.Infof("generating target file: %s", toPath)

	file, err := os.Open(fromPath)
	if err != nil {
		log.Errorf("error opening %s file: %v", toExt, err)
		return
	}
	defer file.Close()

	switch filepath.Ext(fromPath) {
	case ".png":
		si.diskImage, err = png.Decode(file)
		if err != nil {
			log.Errorf("error converting to %s: %v", toExt, err)
			return
		}
	case ".jpg", ".jpeg":
		si.diskImage, err = jpeg.Decode(file)
		if err != nil {
			log.Errorf("error converting to %s: %v", toExt, err)
			return
		}
	}

	ratio := (float64)(si.diskImage.Bounds().Max.Y) / (float64)(si.diskImage.Bounds().Max.X)
	height := int(math.Round(float64(width) * ratio))

	si.finalImage = *image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(&si.finalImage, si.finalImage.Rect, si.diskImage, si.diskImage.Bounds(), draw.Over, nil)

	switch filepath.Ext(toPath) {
	case ".png":
		si.SavePNG(tempPath, toPath)
	case ".jpg", ".jpeg":
		si.SaveJPEG(tempPath, toPath)
	case ".avif":
		si.SaveAVIF(tempPath, toPath)
	case ".webp":
		si.SaveWebp(tempPath, toPath)
	}
}

func ConvertInlineWebp(srcPath string, toDir string, dimensions ...int) string {
	width := 600
	intermediateWidth := 1200

	if len(dimensions) > 0 {
		width = dimensions[0]
	}
	fromDir := filepath.Dir(srcPath)
	start := time.Now()
	hashString := GetFileHash(srcPath)

	outputPath := fmt.Sprintf("%s_%dx.%s.webp",
		strings.TrimSuffix(strings.Replace(srcPath, fromDir, toDir, -1),
			filepath.Ext(srcPath)), width, hashString)

	if !FileExists(outputPath) {
		intermediatePath := fmt.Sprintf("%s_%dx.%s%s",
			strings.TrimSuffix(strings.Replace(srcPath, fromDir, toDir, -1),
				filepath.Ext(srcPath)), intermediateWidth, hashString, filepath.Ext(srcPath))

		// use intermediate if present
		if !FileExists(intermediatePath) {
			var si SafeImage
			si.SaveImage(srcPath, intermediatePath, intermediateWidth)
		}

		if FileExists(outputPath) {
			// log.Info("skipping ", outputPath)
			return outputPath
		}

		var si SafeImage
		si.SaveImage(intermediatePath, outputPath, width)

		log.Infof("(%v) converted image (%s) to webp: %s", time.Since(start), srcPath, outputPath)
	}
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
