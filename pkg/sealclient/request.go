package sealclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const defaultUserAgent = "go-seal-runner-client"
const HttpClientTimeOut = 120

// defaultClient is the client used by default to access the stack. We avoid
// the use of http.DefaultClient which does not have any timeout.
var defaultClient = &http.Client{
	Timeout: 0,
}

type (
	// Error is the typical JSON-API error returned by the API
	Error struct {
		Status string `json:"status"`
		Title  string `json:"title"`
		Detail string `json:"detail"`
	}
)

func (e *Error) Error() string {
	if e.Detail == "" || e.Title == e.Detail {
		return e.Title
	}
	return fmt.Sprintf("%s: %s", e.Title, e.Detail)
}

// ReadJSON reads the content of the specified ReadCloser and closes it.
func ReadJSON(r io.ReadCloser, data interface{}) (err error) {
	defer checkClose(r, &err)
	return json.NewDecoder(r).Decode(&data)
}

// WriteJSON returns an io.Reader from which a JSON encoded data can be read.
func WriteJSON(data interface{}) (io.Reader, error) {
	buf, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(buf), nil
}

func checkClose(c io.Closer, err *error) {
	cerr := c.Close()
	if *err == nil && cerr != nil {
		*err = cerr
	}
}
