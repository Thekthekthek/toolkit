package toolkit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
)

func TestGenerateRandomString(t *testing.T) {
	var tools Tools

	s := tools.GenerateRandomString(10)
	if len(s) != 10 {
		t.Errorf("Length of generated string is not 10: %d", len(s))
	}
}

var uploadTests = []struct {
	name          string
	allowedTypes  []string
	renameFile    bool
	errorExpected bool
}{
	{
		name:          "Allowed no rename",
		allowedTypes:  []string{"image/jpeg", "image/png", "image/gif"},
		renameFile:    false,
		errorExpected: false,
	},
	{
		name:          "Allowed rename",
		allowedTypes:  []string{"image/jpeg", "image/png", "image/gif"},
		renameFile:    true,
		errorExpected: false,
	},
	{
		name:          "not allowed",
		allowedTypes:  []string{"image/jpeg", "image/gif"},
		renameFile:    false,
		errorExpected: true,
	},
}

func TestTools_UploadFiles(t *testing.T) {

	for _, e := range uploadTests {
		pr, pw := io.Pipe()
		writer := multipart.NewWriter(pw)
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer writer.Close()
			defer wg.Done()
			part, err := writer.CreateFormFile("file", "./testdata/img.png")
			if err != nil {
				t.Error(err)
			}
			f, err := os.Open("./testdata/img.png")
			defer f.Close()

			img, _, err := image.Decode(f)
			if err != nil {
				t.Error(err)
			}
			err = png.Encode(part, img)
		}()
		request := httptest.NewRequest("POST", "/", pr)
		request.Header.Add("Content-Type", writer.FormDataContentType())
		var testTools Tools
		testTools.AllowedTypes = e.allowedTypes
		uploadedFiles, err := testTools.UploadFiles(request, "./testdata/uploads", e.renameFile)
		if err != nil && !e.errorExpected {
			t.Error(err)
		}
		if !e.errorExpected {
			if _, err := os.Stat(fmt.Sprintf("./testdata/uploads/%s", uploadedFiles[0].NewFileName)); os.IsNotExist(err) {
				t.Errorf("File %s does not exist", uploadedFiles[0].NewFileName)
			}
			_ = os.Remove(fmt.Sprintf("./testdata/uploads/%s", uploadedFiles[0].NewFileName))
		}
		if !e.errorExpected && err != nil {
			t.Errorf("Error is expected, but got none.")
		}
		wg.Wait()
	}
}

func TestTools_UploadOneFile(t *testing.T) {
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)
	go func() {
		defer writer.Close()
		part, err := writer.CreateFormFile("file", "./testdata/img.png")
		if err != nil {
			t.Error(err)
		}
		f, err := os.Open("./testdata/img.png")
		defer f.Close()

		img, _, err := image.Decode(f)
		if err != nil {
			t.Error(err)
		}
		err = png.Encode(part, img)
	}()
	request := httptest.NewRequest("POST", "/", pr)
	request.Header.Add("Content-Type", writer.FormDataContentType())
	var testTools Tools
	uploadedFile, err := testTools.UploadOneFile(request, "./testdata/uploads", true)
	if err != nil {
		t.Error(err)
	}
	if _, err := os.Stat(fmt.Sprintf("./testdata/uploads/%s", uploadedFile.NewFileName)); os.IsNotExist(err) {
		t.Errorf("File %s does not exist", uploadedFile.NewFileName)
	}
	_ = os.Remove(fmt.Sprintf("./testdata/uploads/%s", uploadedFile.NewFileName))

}

func TestTools_CreateDirIfNotExist(t *testing.T) {
	var testools Tools
	err := testools.CreateDirIfNotExist("./testdata/mydir")
	if err != nil {
		t.Error(err)
	}

	err = testools.CreateDirIfNotExist("./testdata/mydir")
	if err != nil {
		t.Error(err)
	}

	os.Remove("./testdata/mydir")
}

var slugTest = []struct {
	name          string
	s             string
	expected      string
	errorExpected bool
}{
	{
		name:          "valid string",
		s:             "now is the time",
		expected:      "now-is-the-time",
		errorExpected: false,
	},
	{
		name:          "complex string",
		s:             "Now is the time for all GOOD men! + fish & such &^123",
		expected:      "now-is-the-time-for-all-good-men-fish-such-123",
		errorExpected: false,
	},
	{
		name:          "japanese string",
		s:             "ハローワールド",
		expected:      "",
		errorExpected: true,
	},
	{
		name:          "japanese string and roman character",
		s:             "ハhロeーlワlーoルwoドrld",
		expected:      "h-e-l-l-o-wo-rld",
		errorExpected: false,
	},
	{
		name:          "invalid string",
		s:             "",
		expected:      "",
		errorExpected: true,
	},
}

func TestTools_Slugify(t *testing.T) {
	var testools Tools

	for _, e := range slugTest {
		slug, err := testools.Slugify(e.s)
		if err != nil && !e.errorExpected {
			t.Error(err)
		}
		if slug != e.expected && !e.errorExpected {
			t.Errorf("Slug is not '%s': %s", e.expected, slug)
		}
	}
}

func TestTools_DownloadStaticFile(t *testing.T) {
	var testools Tools
	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	testools.DownloadtaticFile(rr, req, "./testdata/", "FileToDownload.go", "pop.go")

	res := rr.Result()
	defer res.Body.Close()
	if res.Header["Content-Length"][0] != "4506" {
		t.Error("Wrong content length, actual length : ", res.Header["Content-Length"][0])
	}
	if res.Header["Content-Disposition"][0] != "attachment; filename=\"pop.go\"" {
		t.Error("Wrong content disposition, actual disposition : ", res.Header["Content-Disposition"][0])
	}

	_, err := io.ReadAll(res.Body)
	if err != nil {
		t.Error(err)
	}
}

var jsonTests = []struct {
	name          string
	json          string
	errorExpected bool
	maxSize       int
	allowUnknown  bool
}{
	{
		name:          "valid json",
		json:          `{"name":"John", "age":30, "car":null}`,
		errorExpected: false,
		maxSize:       1024,
		allowUnknown:  false,
	},
	{
		name:          "badly formatted json",
		json:          `{"name":}`,
		errorExpected: true,
		maxSize:       1024,
		allowUnknown:  false,
	},
	{
		name:          "incorrect type",
		json:          `{"name":"John", "age":"John", "car":null}`,
		errorExpected: true,
		maxSize:       1024,
		allowUnknown:  false,
	},
	{
		name:          "Empty body",
		json:          ``,
		errorExpected: true,
		maxSize:       1024,
		allowUnknown:  false,
	},
	{
		name:          "Syntax error in json",
		json:          `{"name:John"}`,
		errorExpected: true,
		maxSize:       1024,
		allowUnknown:  false,
	},
	{
		name:          "Unknown field in json",
		json:          `{"name":"John", "age":"13", "car":null}`,
		errorExpected: true,
		maxSize:       1024,
		allowUnknown:  false,
	},
	{
		name:          "Two json files",
		json:          `{"name":"John", "age":30, "plane":null}`,
		errorExpected: true,
		maxSize:       1024,
		allowUnknown:  false,
	},
	{
		name:          "Two json files",
		json:          `{"name":"John", "age":30, "plane":null}`,
		errorExpected: false,
		maxSize:       1024,
		allowUnknown:  true,
	},
	{
		name:          "File too large",
		json:          `{"name":"John", "age":30, "plane":null}`,
		errorExpected: true,
		maxSize:       16,
		allowUnknown:  false,
	},
}

func TestTools_ReadJSON(t *testing.T) {
	var testTools Tools

	for _, e := range jsonTests {
		testTools.MaxJSONSize = e.maxSize
		testTools.AllowUnknownFields = e.allowUnknown
		var decodedJSON struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
			Car  string `json:"car"`
		}
		req, err := http.NewRequest("POST", "/", bytes.NewReader([]byte(e.json)))
		if err != nil {
			t.Log("Error :", err)
		}
		rr := httptest.NewRecorder()
		err = testTools.ReadJSON(rr, *req, &decodedJSON)
		if err == nil && e.errorExpected {
			t.Errorf("%s - Error expected, but got none.", e.name)
		}
		if err != nil && !e.errorExpected {
			t.Errorf("%s - Error not expected, but got one : %s", e.name, err.Error())
		}
	}
}

func TestTools_WriteJSON(t *testing.T) {
	var testTools Tools
	rr := httptest.NewRecorder()
	payload := JSONResponse{
		Error:   false,
		Message: "Hello World",
	}
	headers := make(http.Header)
	headers.Add("FOO", "BAR")
	err := testTools.WriteJSON(rr, http.StatusOK, payload, headers)
	if err != nil {
		t.Errorf("Error writing JSON : %s", err.Error())
	}
}

func TestTools_ErrorJSON(t *testing.T) {
	var testTools Tools
	rr := httptest.NewRecorder()
	err := testTools.ErrorJSON(rr, errors.New("some error"), http.StatusServiceUnavailable)
	if err != nil {
		t.Errorf("Error writing JSON : %s", err.Error())
	}
	var payload JSONResponse
	decoder := json.NewDecoder(rr.Body)
	err = decoder.Decode(&payload)
	if err != nil {
		t.Error("Received error when decoding JSON", err)
	}

	if !payload.Error {
		t.Error("Error set to false in JSON, and it should be true")
	}

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Wrong status code returned, expected 503 but got %d", rr.Code)
	}
}
