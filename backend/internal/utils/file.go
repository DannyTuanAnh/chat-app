package utils

import (
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	_ "github.com/gen2brain/avif"
	_ "golang.org/x/image/webp"

	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/h2non/filetype"

	"github.com/google/uuid"
)

var (
	allowExtsEnv = GetEnvList("ALLOWED_EXTENSIONS", []string{})
	allowExts    = SetListBoolean(allowExtsEnv)

	allowMimeTypesEnv = GetEnvList("ALLOWED_MIME_TYPES", []string{})
	allowMimeTypes    = SetListBoolean(allowMimeTypesEnv)

	allowFormatsEnv = GetEnvList("ALLOWED_FORMATS", []string{})
	allowFormats    = SetListBoolean(allowFormatsEnv)
)

var maxImageFileSize = int64(GetEnvInt("MAX_IMAGE_SIZE", 5)) << 20 // 5 MB

func ValidateAndReturnObjNameImage(userID int32, fileHeader *multipart.FileHeader) (multipart.File, string, error) {
	// 1. Validate filename - prevent path traversal and invalid characters
	// Prevent path separator (both / and \)
	if strings.ContainsAny(fileHeader.Filename, "/\\") {
		return nil, "", errors.New("filename contains path separator characters")
	}

	// Prevent path traversal
	if strings.Contains(fileHeader.Filename, "..") {
		return nil, "", errors.New("filename contains path traversal sequences")
	}

	// Prevent special characters that can cause issues in file systems
	if strings.ContainsAny(fileHeader.Filename, `<>:"|?*`) {
		return nil, "", errors.New("filename contains invalid special characters")
	}

	// 2. Check extension in file name
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if !allowExts[ext] {
		return nil, "", fmt.Errorf("unsupported file extension: (%s)", ext)
	}

	// 3. Check file size
	if fileHeader.Size > maxImageFileSize {
		return nil, "", errors.New("File is too large (less than 5 MB)")
	}

	// 4. Open file
	file, err := fileHeader.Open()
	if err != nil {
		return nil, "", fmt.Errorf(`unable to open file: %v`, err)
	}
	defer file.Close()

	// 5. Read file content
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)

	if err != nil && err != io.EOF {
		return nil, "", fmt.Errorf("unable to read file: %v", err)
	}

	if n == 0 {
		return nil, "", errors.New("file is empty")
	}

	// 6. Validate image content
	kind, err := filetype.Match(buffer[:n])
	if err != nil {
		return nil, "", fmt.Errorf("cannot detect file type: %v", err)
	}

	if kind.MIME.Type != "image" {
		return nil, "", fmt.Errorf("file is not an image (detected MIME type: %s)", kind.MIME.Value)
	}

	// Check nếu không detect được định dạng
	if kind == filetype.Unknown {
		return nil, "", errors.New("unknown file type")
	}

	// 7. Validate MIME type
	if !allowMimeTypes[kind.MIME.Value] {
		return nil, "", fmt.Errorf("unsupported MIME type: %s (detected: %s)", kind.MIME.Value, kind.Extension)
	}

	// 8. Validate image true
	// This step decodes the image to ensure it is a valid image format.
	// If the image cannot be decoded, it is likely corrupted or not a valid image.
	// If the image is valid, it returns nil.
	// If the image is corrupted or invalid, it returns an error.
	// This step is important to ensure that the uploaded file is a valid image.

	// Reset file pointer to the beginning before validating the image content
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, "", fmt.Errorf("cannot reset file pointer for validating: %v", err)
	}
	if err := validateImageTrue(file); err != nil {
		return nil, "", fmt.Errorf("corrupted or invalid image file: %v", err)
	}

	// Change file name
	objectName := fmt.Sprintf("%d_%s", userID, uuid.New().String())

	// 9. Return multipart.File and objectName
	// Reset file pointer to the beginning before saving the file
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, "", fmt.Errorf("cannot reset file pointer for saving: %v", err)
	}

	return file, objectName, nil
}

// validateImageTrue func checks if the image data is a valid image format.
// It decodes the image and checks if the format is allowed.
// If the image cannot be decoded or the format is not allowed, it returns an error.
// If the image is valid, it returns nil.
func validateImageTrue(r io.Reader) error {
	_, format, err := image.Decode(r)

	if err != nil {
		return fmt.Errorf("cannot decode image: %v", err)
	}

	if !allowFormats[format] {
		return fmt.Errorf("unsupported image format: %s", format)
	}

	return nil
}

// saveFile func saves the uploaded file to the specified destination.
func saveFile(src io.Reader, destination string) error {
	out, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, src); err != nil {
		os.Remove(destination)
		return err
	}

	return err
}
