package graphqlupload

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type postedFileCollection func(key string) (multipart.File, *multipart.FileHeader, error)
type params struct {
	OperationName string                 `json:"operationName"`
	Variables     interface{}            `json:"variables"`
	Query         interface{}            `json:"query"`
	Operations    map[string]interface{} `json:"operations"`
	Map           map[string][]string    `json:"map"`
}
type fileOperation struct {
	Fields         interface{}
	FileCollection postedFileCollection
	MapEntryIndex  string
	SplittedPath   []string
}

type graphqlUploadError struct {
	errorString string
}

func (e graphqlUploadError) Error() string {
	return e.errorString
}

var (
	errMissingOperationsParam      = &graphqlUploadError{"Missing operations parameter"}
	errMissingMapParam             = &graphqlUploadError{"Missing operations parameter"}
	errInvalidMapParam             = &graphqlUploadError{"Invalid map parameter"}
	errIncompleteRequestProcessing = &graphqlUploadError{"Could not process request"}
	addFileChannel                 = make(chan fileOperation)
)

var mapEntries map[string][]string
var singleOperations map[string]interface{}
var wg sync.WaitGroup

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
						http.Error(w, errMissingOperationsParam.Error(), http.StatusBadRequest)
						return
					}
					if &m == nil {
						http.Error(w, errMissingMapParam.Error(), http.StatusBadRequest)
						return
					}
					err := json.Unmarshal([]byte(o), &singleOperations)
					if err == nil {
						err = json.Unmarshal([]byte(m), &mapEntries)
						if err == nil {
							mo, err := singleTransformation(mapEntries, singleOperations, r.FormFile)
							if err != nil {
								http.Error(w, errIncompleteRequestProcessing.Error(), http.StatusBadRequest)
								return
							}
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
							http.Error(w, errInvalidMapParam.Error(), http.StatusBadRequest)
							return
						}

					} else {
						var batchOperations []map[string]interface{}
						err := json.Unmarshal([]byte(o), &batchOperations)
						if err == nil {
							if err := json.Unmarshal([]byte(m), &mapEntries); err == nil {
								_ = batchTransformation(mapEntries, batchOperations, r.FormFile)
								// p := params{
								// 	OperationName: r.PostFormValue("operationName"),
								// 	Variables:     mo["variables"],
								// 	Query:         mo["query"],
								// 	Operations:    singleOperations,
								// 	Map:           mapEntries,
								// }
								// body, err := json.Marshal(p)
								// if err == nil {
								// 	r.Body = ioutil.NopCloser(bytes.NewReader(body))
								// 	w.Header().Set("Content-Type", "application/json")
								// }
							}
						}
					}

				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func singleTransformation(mapEntries map[string][]string, operations map[string]interface{}, p postedFileCollection) (map[string]interface{}, error) {

	for idx, mapEntry := range mapEntries {
		for _, entry := range mapEntry {
			wg.Add(1)
			go func(entry, idx string, operations map[string]interface{}) {
				defer wg.Done()

				wg.Add(1)
				go func() {
					defer wg.Done()
					_, _ = addFile()
				}()

				entryPaths := strings.Split(entry, ".")
				fields := findField(operations, entryPaths[:len(entryPaths)-1])

				addFileChannel <- fileOperation{
					Fields:         fields,
					FileCollection: p,
					MapEntryIndex:  idx,
					SplittedPath:   entryPaths,
				}
			}(entry, idx, operations)

		}
	}
	wg.Wait()

	return operations, nil
}
func batchTransformation(mapEntries map[string][]string, batchOperations []map[string]interface{}, p postedFileCollection) []map[string]interface{} {
	for _, mapEntry := range mapEntries {
		for _, entry := range mapEntry {
			entryPaths := strings.Split(entry, ".")
			opIdx, _ := strconv.Atoi(entryPaths[0])
			operations := batchOperations[opIdx]
			_ = findField(operations, entryPaths[:len(entryPaths)-1])
			// _ = addFileToOperations(fields, p, idx, entryPaths)

		}
	}
	return batchOperations
}
func findField(operations interface{}, entryPaths []string) map[string]interface{} {
	for i := 0; i < len(entryPaths); i++ {
		if arr, ok := operations.([]interface{}); ok {
			index, err := strconv.Atoi(entryPaths[i])
			if err != nil {
				panic("non-integer index provided for array value")
			}
			operations = arr[index]
		} else if op, ok := operations.(map[string]interface{}); ok {
			operations = op[entryPaths[i]]
		} else {
			panic("invalid operation mapping")
		}
	}
	return operations.(map[string]interface{})
}

func addFile() (interface{}, error) {
	params := <-addFileChannel
	file, handle, err := params.FileCollection(params.MapEntryIndex)
	if err != nil {
		return nil, fmt.Errorf("could not access multipart file. reason: %v", err)
	}
	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("could not read multipart file. reason: %v", err)
	}
	f, err := ioutil.TempFile(os.TempDir(), fmt.Sprintf("graphql-upload-%s%s", handle.Filename, filepath.Ext(handle.Filename)))
	if err != nil {
		return nil, fmt.Errorf("unable to create temp file. reason: %v", err)
	}

	_, err = f.Write(data)
	if err != nil {
		return nil, fmt.Errorf("could not write file. reason: %v", err)
	}
	mimeType := handle.Header.Get("Content-Type")
	if op, ok := params.Fields.([]map[string]interface{}); ok {
		fidx, _ := strconv.Atoi(params.SplittedPath[len(params.SplittedPath)-1])
		upload := &GraphQLUpload{
			MIMEType: mimeType,
			FileName: handle.Filename,
			FilePath: f.Name(),
		}
		fmt.Printf("%#v", *upload)
		op[fidx][params.SplittedPath[len(params.SplittedPath)-1]] = upload
		return op, nil
	} else if op, ok := params.Fields.(map[string]interface{}); ok {
		op[params.SplittedPath[len(params.SplittedPath)-1]] = &GraphQLUpload{
			MIMEType: mimeType,
			FileName: handle.Filename,
			FilePath: f.Name(),
		}
		return op, nil
	}
	return nil, nil
}
