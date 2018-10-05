package graphqlupload

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type postedFileCollection func(key string) (multipart.File, *multipart.FileHeader, error)
type params struct {
	OperationName string                 `json:"operationName"`
	Variables     interface{}            `json:"variables"`
	Query         interface{}            `json:"query"`
	Operations    map[string]interface{} `json:"operations"`
	Map           map[string][]string    `json:"map"`
}
type graphqlUploadError struct {
	errorString string
}

func (e graphqlUploadError) Error() string {
	return e.errorString
}

var (
	MissingOperationsParam = &graphqlUploadError{"Missing operations parameter"}
	MissingMapParam        = &graphqlUploadError{"Missing operations parameter"}
	InvalidMapParam        = &graphqlUploadError{"Invalid map parameter"}
)

var mapEntries map[string][]string
var singleOperations map[string]interface{}

func Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			v := r.Header.Get("Content-Type")
			if v != "" {
				mediatype, _, _ := mime.ParseMediaType(v)
				if mediatype == "multipart/form-data" {
					r.ParseMultipartForm((1 << 20) * 64)
					m := r.PostFormValue("map")
					o := r.PostFormValue("operations")
					if &o == nil {
						http.Error(w, MissingOperationsParam.Error(), http.StatusBadRequest)
						return
					}
					if &m == nil {
						http.Error(w, MissingMapParam.Error(), http.StatusBadRequest)
						return
					}
					err := json.Unmarshal([]byte(o), &singleOperations)
					if err == nil {
						err = json.Unmarshal([]byte(m), &mapEntries)
						if err == nil {
							mo := singleTransformation(mapEntries, singleOperations, r.FormFile)
							p := params{
								OperationName: r.PostFormValue("operationName"),
								Variables:     mo["variables"],
								Query:         mo["query"],
								Operations:    singleOperations,
								Map:           mapEntries,
							}
							body, err := json.Marshal(p)
							if err == nil {
								r.Body = ioutil.NopCloser(bytes.NewReader(body))
								w.Header().Set("Content-Type", "application/json")
							}
						} else {
							http.Error(w, InvalidMapParam.Error(), http.StatusBadRequest)
							return
						}

					} else {
						var batchOperations []map[string]interface{}
						err := json.Unmarshal([]byte(o), &batchOperations)
						if err == nil {
							if err := json.Unmarshal([]byte(m), &mapEntries); err == nil {
								_ = multipleTransformation(mapEntries, batchOperations, r.FormFile)
							}
						}
					}

				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func singleTransformation(mapEntries map[string][]string, operations map[string]interface{}, p postedFileCollection) map[string]interface{} {
	for idx, mapEntry := range mapEntries {
		for _, entry := range mapEntry {
			entryPaths := strings.Split(entry, ".")
			fields := findField(operations, entryPaths[:len(entryPaths)-1])
			addFileToOperations(fields, p, idx, entryPaths)
		}
	}
	return operations
}
func multipleTransformation(mapEntries map[string][]string, batchOperations []map[string]interface{}, p postedFileCollection) []map[string]interface{} {
	for idx, mapEntry := range mapEntries {
		for _, entry := range mapEntry {
			entryPaths := strings.Split(entry, ".")
			operationIndex, _ := strconv.Atoi(entryPaths[0])
			operations := batchOperations[operationIndex]
			fields := findField(operations, entryPaths[:len(entryPaths)-1])
			batchOperations[operationIndex] = addFileToOperations(fields, p, idx, entryPaths)
		}
	}
	return batchOperations
}
func findField(operations interface{}, entryPaths []string) map[string]interface{} {
	for i := 0; i < len(entryPaths); i++ {
		if arr, ok := operations.([]map[string]interface{}); ok {
			operations = arr[i]
			return findField(operations, entryPaths)
		} else if op, ok := operations.(map[string]interface{}); ok {
			operations = op[entryPaths[i]]
		}
	}
	return operations.(map[string]interface{})
}
func addFileToOperations(operations map[string]interface{}, p postedFileCollection, idx string, entryPaths []string) map[string]interface{} {
	file, handle, err := p(idx)
	if err != nil {
		log.Printf("could not access multipart file. reason: %v", err)
		return operations
	}
	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		log.Printf("could not read multipart file. reason: %v", err)
		return operations
	}
	name := strings.Join([]string{os.TempDir(), handle.Filename}, "/")
	err = ioutil.WriteFile(name, data, 0666)
	if err != nil {
		log.Printf("could not write file. reason: %v", err)
		return operations
	}
	mimeType := handle.Header.Get("Content-Type")
	operations[entryPaths[len(entryPaths)-1]] = &GraphQLUpload{
		MIMEType: mimeType,
		Filename: handle.Filename,
		Filepath: name,
	}
	return operations
}

func addFile(operations interface{}, p postedFileCollection, idx string, entryPaths []string) interface{} {
	file, handle, err := p(idx)
	if err != nil {
		log.Printf("could not access multipart file. reason: %v", err)
		return operations
	}
	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		log.Printf("could not read multipart file. reason: %v", err)
		return operations
	}
	name := strings.Join([]string{os.TempDir(), handle.Filename}, "/")
	err = ioutil.WriteFile(name, data, 0666)
	if err != nil {
		log.Printf("could not write file. reason: %v", err)
		return operations
	}
	mimeType := handle.Header.Get("Content-Type")

	if op, ok := operations.([]map[string]interface{}); ok {
		fidx, _ := strconv.Atoi(entryPaths[len(entryPaths)-1])
		// op[fidx] = &GraphQLUpload{
		// 	MIMEType: mimeType,
		// 	Filename: handle.Filename,
		// 	Filepath: name,
		// }
		fmt.Printf("%#v %#v", op, fidx)
	} else if op, ok := operations.(map[string]interface{}); ok {
		op[entryPaths[len(entryPaths)-1]] = &GraphQLUpload{
			MIMEType: mimeType,
			Filename: handle.Filename,
			Filepath: name,
		}
	}
	return operations
}
