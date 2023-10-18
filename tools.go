package toolkit

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

type Tools struct {
	MaxFileSize        int64
	AllowedTypes       []string
	MaxJSONSize        int
	AllowUnknownFields bool
}

const randomStringSource = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz_+"

func (t *Tools) GenerateRandomString(n int) string {
	s, r := make([]rune, n), []rune(randomStringSource)
	for i := range s {
		p, _ := rand.Prime(rand.Reader, len(r))
		x, y := p.Uint64(), uint64(len(r))
		s[i] = r[x%y]
	}
	return string(s)
}

type UploadedFile struct {
	NewFileName      string
	OriginalFileName string
	FileSize         int64
}

func (t *Tools) UploadOneFile(r *http.Request, uploadDir string, rename ...bool) (*UploadedFile, error) {
	renameFile := true
	if len(rename) > 0 {
		renameFile = rename[0]
	}
	files, err := t.UploadFiles(r, uploadDir, renameFile)
	if err != nil {
		return nil, err
	}
	return files[0], err
}

func (t *Tools) UploadFiles(r *http.Request, uploadDir string, rename ...bool) ([]*UploadedFile, error) {
	renameFile := true
	if len(rename) > 0 {
		renameFile = rename[0]
	}
	var uploadedFiles []*UploadedFile

	if t.MaxFileSize == 0 {
		t.MaxFileSize = 1024 * 1024 * 1024
	}

	err := t.CreateDirIfNotExist(uploadDir)
	if err != nil {
		return nil, err
	}
	err = r.ParseMultipartForm(t.MaxFileSize)
	if err != nil {
		return nil, errors.New("File size is too big")
	}

	for _, fHeaders := range r.MultipartForm.File {
		for _, hdr := range fHeaders {
			uploadedFiles, err = func([]*UploadedFile) ([]*UploadedFile, error) {
				var uploadedFile UploadedFile
				infile, err := hdr.Open()
				if err != nil {
					return nil, err
				}
				defer infile.Close()
				buff := make([]byte, 512)
				_, err = infile.Read(buff)
				if err != nil {
					return nil, err
				}
				allowed := false
				filetype := http.DetectContentType(buff)
				if len(t.AllowedTypes) > 0 {
					for _, x := range t.AllowedTypes {
						if strings.EqualFold(x, filetype) {
							allowed = true
							break
						}
					}
				} else {
					allowed = true
				}
				if !allowed {
					return nil, errors.New("File type is not allowed")
				}

				_, err = infile.Seek(0, 0)
				if err != nil {
					return nil, err
				}
				if renameFile {
					uploadedFile.NewFileName = fmt.Sprintf("%s%s", t.GenerateRandomString(25), filepath.Ext(hdr.Filename))
				} else {
					uploadedFile.NewFileName = hdr.Filename
				}
				uploadedFile.OriginalFileName = hdr.Filename
				var outfile *os.File
				defer outfile.Close()

				if outfile, err = os.Create(filepath.Join(uploadDir, uploadedFile.NewFileName)); err != nil {
					return nil, err
				} else {
					fileSize, err := io.Copy(outfile, infile)
					if err != nil {
						return nil, err
					}
					uploadedFile.FileSize = fileSize
				}
				uploadedFiles = append(uploadedFiles, &uploadedFile)
				return uploadedFiles, nil
			}(uploadedFiles)
			if err != nil {
				return uploadedFiles, err
			}
		}
	}
	return uploadedFiles, nil
}

func (t *Tools) CreateDirIfNotExist(path string) error {
	const mode = 0755
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := os.MkdirAll(path, mode)
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *Tools) Slugify(s string) (string, error) {
	if s == "" {
		return "", errors.New("String is empty")
	}
	re := regexp.MustCompile(`[^a-z\d]+`)
	slug := strings.Trim(re.ReplaceAllString(strings.ToLower(s), "-"), "-")
	if len(slug) == 0 {
		return "", errors.New("Slug is empty")
	}
	return slug, nil
}

func (t *Tools) DownloadtaticFile(w http.ResponseWriter, r *http.Request, p, file, displayName string) {
	filePath := path.Join(p, file)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", displayName))
	http.ServeFile(w, r, filePath)
}

type JSONResponse struct {
	Error   bool        `json:"error"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (t *Tools) ReadJSON(w http.ResponseWriter, r http.Request, data interface{}) error {
	maxBytes := 1024 ^ 2
	if t.MaxJSONSize == 0 {
		maxBytes = t.MaxJSONSize
	}
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))
	dec := json.NewDecoder(r.Body)
	if !t.AllowUnknownFields {
		dec.DisallowUnknownFields()
	}
	err := dec.Decode(&data)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshallError *json.InvalidUnmarshalError
		switch {
		case errors.As(err, &syntaxError):
			return fmt.Errorf("Request body contains badly-formed JSON (at position %d)", syntaxError.Offset)
		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("Body contains badly-formed JSON")
		case errors.As(err, &syntaxError):
			if unmarshalTypeError.Field != "" {
				return fmt.Errorf("Request body contains an invalid value for the %q field (at position %d)", unmarshalTypeError.Field, syntaxError.Offset)
			}
			return fmt.Errorf("Request body contains an invalid value (at position %d)", syntaxError.Offset)
		case errors.Is(err, io.EOF):
			return errors.New("Request body must not be empty")
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			return fmt.Errorf("Request body contains unknown field %s", fieldName)
		case err.Error() == "http: request body too large":
			return fmt.Errorf("Body must not be larger than %d bytes", maxBytes)
		case errors.As(err, &invalidUnmarshallError):
			return fmt.Errorf("Error unmarshalling the JSON : %s", err.Error())
		default:
			return err
		}
	}

	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		return errors.New("Request body must only contain a single JSON object")
	}
	return nil
}

func (t *Tools) WriteJSON(w http.ResponseWriter, status int, data interface{}, headers ...http.Header) error {
	out, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if len(headers) > 0 {
		for key, value := range headers[0] {
			w.Header()[key] = value
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, err = w.Write(out)
	if err != nil {
		return err
	}
	return nil
}

func (t *Tools) ErrorJSON(w http.ResponseWriter, err error, status ...int) error {
	statusCode := http.StatusBadRequest
	if len(status) > 0 {
		statusCode = status[0]
	}
	var payload JSONResponse
	payload.Error = true
	payload.Message = err.Error()
	return t.WriteJSON(w, statusCode, payload)
}
