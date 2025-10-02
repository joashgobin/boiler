package helpers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"io/ioutil"
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/google/uuid"
	"github.com/kolesa-team/go-webp/encoder"
	"github.com/kolesa-team/go-webp/webp"
	"github.com/spf13/viper"
	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
	"github.com/tdewolff/minify/v2/js"
)

const MySQLTimestamp = "2006-01-02 15:04:05"

func GetRandomUUID() string {
	randomUUID, err := uuid.NewRandom()
	if err != nil {
		return ""
	}
	return randomUUID.String()
}

func WasteTime(numSeconds int) {
	var duration time.Duration
	duration = time.Duration(numSeconds) * time.Second
	start := time.Now()
	for time.Since(start) < duration {
		time.Sleep(time.Duration(rand.Intn(50)) * time.Millisecond)
	}
}

func Background(fn func(), wg *sync.WaitGroup) {
	wg.Go(func() {
		fn()
	})
	/*
		go func() {
			defer func() {
				if err := recover(); err != nil {
					log.Error(fmt.Sprintf("%v", err))
				}
			}()

			fn()
		}()
	*/
}

func PrintType(v interface{}) {
	switch v := v.(type) {
	case int:
		fmt.Printf("Value %d is of type int\n", v)
	case string:
		fmt.Printf("Value %q is of type string\n", v)
	case float64:
		fmt.Printf("Value %f is of type float64\n", v)
	default:
		fmt.Printf("Value %v is of type %T\n", v, v)
	}
}

// helper to be used in template engine to get reference to file
/*
func GetFingerprint(staticDir string) string {
	fullPath := filepath.Join(staticDir, strings.TrimPrefix(path, "/static"))
	fileInfo, err := os.Stat(fullPath)
	fp, err := GenerateFingerprint(fullPath)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	newURL := fmt.Sprintf("%s.%s%s",
		strings.TrimSuffix(path, filepath.Ext(path)),
		fp,
		filepath.Ext(path))
	log.Infof("generated fingerprint %s", newURL)
}
*/

func GenerateFingerprint(srcPath string, fileListPtr *map[string]string) (string, error) {
	err := os.MkdirAll("static/gen", 0755)
	if err != nil {
		log.Infof("failed to create directory", "static/gen")
	}

	srcContent, err := os.ReadFile(srcPath)
	if err != nil {
		return "", err
	}

	m := minify.New()
	m.AddFunc("text/css", css.Minify)
	m.AddFunc("text/javascript", js.Minify)

	// minify the file first e.g. style.min.css
	minPath := fmt.Sprintf("%s.min%s",
		strings.TrimSuffix(strings.Replace(srcPath, "static/", "static/gen/", -1), filepath.Ext(srcPath)),
		filepath.Ext(srcPath))

	mimeType := GetMimeType(srcPath)
	minifiedContent, err := m.Bytes(mimeType, srcContent)
	if err != nil {
		return "", err
	}

	hashString := FingerprintFromBuffer(minifiedContent)

	dstPath := fmt.Sprintf("%s.min.%s%s",
		strings.TrimSuffix(strings.Replace(srcPath, "static/", "static/gen/", -1), filepath.Ext(srcPath)),
		hashString,
		filepath.Ext(srcPath))

	if FileExists(dstPath) {
		(*fileListPtr)[strings.TrimPrefix(srcPath, "static/")] = dstPath
		return dstPath, nil
	}

	if err := os.WriteFile(dstPath, minifiedContent, 0644); err != nil {
		return "", err
	}

	log.Infof("minified file (%s) to new file: %s", minPath, dstPath)
	// map src path to dest path
	(*fileListPtr)[strings.TrimPrefix(srcPath, "static/")] = dstPath

	return dstPath, nil
}

func GetMimeType(path string) string {
	switch {
	case strings.HasSuffix(path, ".css"):
		return "text/css"
	case strings.HasSuffix(path, ".js"):
		return "text/javascript"
	default:
		return "text/plain"
	}
}

func FingerprintFromBuffer(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

func GetHash(content string) string {
	hashBytes := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hashBytes[:])
}

func ParseBodyForKey(bodyData []byte, key string) map[string]string {
	body := string(bodyData)
	// Split the body into individual key-value pairs
	pairs := strings.Split(body, "&")

	// Create a map to store the key-value pairs
	data := make(map[string]string)

	// Process each pair
	for _, pair := range pairs {
		// Split each pair into key and value
		kv := strings.Split(pair, "=")

		// Skip invalid pairs
		if len(kv) != 2 {
			continue
		}

		if strings.Contains(kv[0], key) {
			// Store the key-value pair
			data[kv[0]] = kv[1]
		}
	}

	return data
}

func CompileFromBody(bodyData []byte, key string) []string {
	body := string(bodyData)
	// fmt.Printf("%v\n", body)

	pairs := strings.Split(body, "&")

	var data []string

	for _, pair := range pairs {
		kv := strings.Split(pair, "=")

		if len(kv) != 2 {
			continue
		}

		if strings.Contains(kv[0], key) {
			data = append(data, strings.ReplaceAll(kv[1], "+", " "))
		}
	}
	return data
}

func CollectFiberFormData(c *fiber.Ctx, fields *[]string, multiples *[]string) string {
	var snippets string
	for _, field := range *fields {
		if slices.Contains(*multiples, field) {
			// fmt.Printf("%s\n", field)
			values := CompileFromBody(c.Body(), "options-"+ReplaceSpecial(field))
			snippets = snippets + "<p><strong>" + field + "</strong>:<ul>"
			for _, value := range values {
				snippets = snippets + "<li>" + value + "</li>"
			}
			snippets = snippets + "</ul></p>"
		} else {
			snippets = snippets + "<p><strong>" + field + "</strong>: " + c.FormValue(ReplaceSpecial(field)) + "</p>"
		}
	}
	return snippets
}

func MapFromFormBody(c *fiber.Ctx, excludeEmpty bool) map[string]string {
	body := string(c.Body())

	pairs := strings.Split(body, "&")

	data := make(map[string]string, 1)

	for _, pair := range pairs {
		kv := strings.Split(pair, "=")

		if len(kv) != 2 {
			continue
		}

		if kv[0] == "csrf" {
			continue
		}

		if kv[1] == "" && excludeEmpty {
			continue
		}

		value, err := url.QueryUnescape(kv[1])
		if err == nil {
			data[kv[0]] = value
		}
	}
	return data
}

func EnsureFiberFormFields(c *fiber.Ctx, fields []string) (string, error) {
	for _, v := range fields {
		if c.FormValue(v) == "" || len(strings.Trim(c.FormValue(v), " ")) == 0 {
			return fmt.Sprintf("Please input %s", v), fmt.Errorf("form: value missing: %s", v)
		}
	}
	return "", nil
}

func ReplaceSpecial(text string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9]`)
	return strings.ToLower(re.ReplaceAllString(text, "-"))
}

func ConvertToWebp(srcPath string, fileListPtr *map[string]string, fromDir, toDir string) error {
	outputPath := fmt.Sprintf("%s.webp",
		strings.TrimSuffix(strings.Replace(srcPath, fromDir, toDir, -1),
			filepath.Ext(srcPath)))
	if FileExists(outputPath) {
		(*fileListPtr)[strings.TrimPrefix(srcPath, "static/")] = outputPath
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
	log.Infof("converted image (%s) to webp: %s", srcPath, outputPath)
	(*fileListPtr)[strings.TrimPrefix(srcPath, "static/")] = outputPath
	return nil
}

func GenerateFingerprintsForFolder(folderPath string, targetFolder string, ext string, fileListPtr *map[string]string) {
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
			_, err = GenerateFingerprint(filepath.Join(folderPath, entry.Name()), fileListPtr)
			if err != nil {
				log.Errorf("could not generate fingerprint for file (%s): %v", entry.Name(), err)
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

func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// helper to create a database connection pool
func OpenDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func CreateDirectory(path string) error {
	err := os.MkdirAll(path, 0755)
	if err != nil {
		return err
	}
	return nil
}

func CopyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		// Create destination directory
		destPath := filepath.Join(dst, relPath)
		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		// Copy file
		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		destFile, err := os.Create(destPath)
		if err != nil {
			return err
		}
		defer destFile.Close()

		if _, err := io.Copy(destFile, srcFile); err != nil {
			return err
		}

		return destFile.Chmod(info.Mode())
	})
}

func TouchFile(filePath string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	return nil
}

func SaveTextToDirectory(text string, filePath string) error {
	if text == "" || filePath == "" {
		return fmt.Errorf("text content and filePath must not be empty")
	}
	text = strings.TrimSpace(text)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	err := ioutil.WriteFile(filePath, []byte(text), 0644)
	if err != nil {
		return fmt.Errorf("failed to write text to file: %v", err)
	}
	// log.Infof("saved text to file: %s", filePath)
	return nil
}

func RunMigration(migrationQuery string, db *sql.DB) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := db.ExecContext(ctx, migrationQuery)
	if err != nil {
		var mySQLError *mysql.MySQLError
		if errors.As(err, &mySQLError) {
			if mySQLError.Number == 1064 {
				log.Errorf("error in migration: %v", err)
				return
			} else {
				log.Errorf("error in migration: %v", err)
				return
			}
		}
		return
	}

	/*
		if err != nil {
			log.Errorf("failed to run migration: %v", err)
		}
	*/
	_, err = result.RowsAffected()
	if err != nil {
		log.Errorf("failed to run migration: %v", err)
	}
	// log.Infof("migration executed, rows affected: %d", rowsAffected)
}

func MigrateUp(db *sql.DB, migrationQuery string, args map[string]string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	finalMigrationQuery := migrationQuery
	for key, value := range args {
		finalMigrationQuery = strings.ReplaceAll(finalMigrationQuery, "<"+key+">", value)
	}
	result, err := db.ExecContext(ctx, finalMigrationQuery)
	if err != nil {
		var mySQLError *mysql.MySQLError
		if errors.As(err, &mySQLError) {
			if mySQLError.Number == 1064 {
				log.Errorf("error in migration: %v", err)
				return
			} else {
				log.Errorf("error in migration: %v", err)
				return
			}
		}
		return
	}

	/*
		if err != nil {
			log.Errorf("failed to run migration: %v", err)
		}
	*/
	_, err = result.RowsAffected()
	if err != nil {
		log.Errorf("failed to run migration: %v", err)
	}
	// log.Infof("migration executed, rows affected: %d", rowsAffected)
}

func StructsToMaps(structs interface{}) []map[string]interface{} {
	// Convert input to slice
	rv := reflect.ValueOf(structs)
	if rv.Kind() != reflect.Slice {
		return []map[string]interface{}{}
	}

	result := make([]map[string]interface{}, rv.Len())

	// Process each struct in the slice
	for i := 0; i < rv.Len(); i++ {
		elem := rv.Index(i)
		if elem.Kind() != reflect.Struct {
			continue
		}

		// Create map for this struct
		m := make(map[string]interface{})

		// Add all exported fields to map
		for j := 0; j < elem.NumField(); j++ {
			field := elem.Field(j)
			if field.CanInterface() {
				m[elem.Type().Field(j).Name] = field.Interface()
			}
		}

		result[i] = m
	}

	return result
}

func Getenv(key string) string {
	viper.SetConfigName("config")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()

	if err != nil {
		log.Errorf("error reading config.env file: %v", err)
	}

	val := viper.GetString(strings.ToLower(key))
	// log.Infof("settings: %v", viper.AllSettings())
	// log.Infof("env %s: %s", key, val)
	if !viper.IsSet(key) {
		log.Warnf("env var not set: %s", key)
	}
	return val
}

func ConvertPNGToJPG(inputPath, outputPath string) {
	pngBytes, err := os.ReadFile(inputPath)
	if err != nil {
		log.Errorf("Error reading PNG file: %v", err)
		return
	}
	// Decode the PNG image
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		log.Errorf("failed to decode PNG: %v", err)
		return
	}

	// Create a buffer for the JPG output
	buf := new(bytes.Buffer)

	// Encode as JPG with default quality
	if err := jpeg.Encode(buf, img, &jpeg.Options{Quality: 95}); err != nil {
		log.Errorf("failed to encode JPG: %v", err)
		return
	}

	if err := os.WriteFile(outputPath, buf.Bytes(), 0644); err != nil {
		log.Fatalf("Failed to write JPG file: %v", err)
		return
	}
}
