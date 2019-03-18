package graphqlupload

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type GraphQLUpload struct {
	FileName string `json:"filename"`
	MIMEType string `json:"mimetype"`
	FilePath string `json:"filepath"`
}

func (_ GraphQLUpload) ImplementsGraphQLType(name string) bool { return name == "Upload" }
func (u *GraphQLUpload) UnmarshalGraphQL(input interface{}) error {
	switch input := input.(type) {
	case map[string]interface{}:
		b, err := json.Marshal(input)
		if err != nil {
			u = &GraphQLUpload{}
		} else {
			json.Unmarshal(b, u)
		}
		return nil
	default:
		return fmt.Errorf("no implementation for the type specified")
	}
}
func createReadStream(u *GraphQLUpload) (io.Reader, error) {
	f, err := os.Open(u.FilePath)
	if err == nil {
		return bufio.NewReader(f), nil
	}
	return nil, err
}
func (u *GraphQLUpload) CreateReadStream() (io.Reader, error) {
	return createReadStream(u)
}

func (u *GraphQLUpload) WriteFile(name string) error {
	rdr, err := createReadStream(u)
	if err != nil {
		return err
	}
	fo, err := os.Create(name)
	if err != nil {
		return err
	}
	defer func() {
		if err := fo.Close(); err != nil {
			panic(err)
		}
	}()
	w := bufio.NewWriter(fo)
	buf := make([]byte, 1024*1024)
	for {
		n, err := rdr.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			break
		}
		if _, err := w.Write(buf[:n]); err != nil {
			return err
		}
	}

	if err = w.Flush(); err != nil {
		return err
	}
	return nil
}
