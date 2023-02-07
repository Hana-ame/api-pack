package missakujo

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"log"
	"net/http"
)

func getApis(
	host string,
	token string,
	userId string,
) (
	func(since, until int64) ([]byte, error),
	func(noteId string) ([]byte, error),
	func(noteId string) ([]byte, error),
) {
	listNotesEndpoint := `https://` + host + `/api/users/notes`
	showNotesEndpoint := `https://` + host + `/api/notes/show`
	deleteNotesEndpoint := `https://` + host + `/api/notes/delete`
	deleteNotes := func(noteId string) ([]byte, error) {
		data := make(map[string]any)
		data["noteId"] = noteId
		data["i"] = token
		body, err := json.Marshal(data)
		if err != nil {
			log.Println(`Marshal Error: `, err)
			return nil, err
		}
		return fetchHandler(deleteNotesEndpoint, body)
	}
	showNotes := func(noteId string) ([]byte, error) {
		data := make(map[string]any)
		data["noteId"] = noteId
		data["i"] = token
		body, err := json.Marshal(data)
		if err != nil {
			log.Println(`Marshal Error: `, err)
			return nil, err
		}
		return fetchHandler(showNotesEndpoint, body)
	}
	listNotes := func(since, until int64) ([]byte, error) {
		data := make(map[string]any)
		data["userId"] = userId
		data["sinceDate"] = since
		data["untilDate"] = until
		data["i"] = token
		body, err := json.Marshal(data)
		if err != nil {
			log.Println(`Marshal Error: `, err)
			return nil, err
		}
		return fetchHandler(listNotesEndpoint, body)
	}

	return listNotes, showNotes, deleteNotes
}

var Client *http.Client = &http.Client{}

func fetchHandler(url string, body []byte) ([]byte, error) {
	log.Println("fetch", url, string(body))
	req, err := http.NewRequest(
		http.MethodPost,
		url,
		bytes.NewReader(body),
	)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := Client.Do(req)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	reader := getPlainTextReader(resp.Body, resp.Header.Get("Content-Encoding"))
	payload, err := io.ReadAll(reader)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	log.Println("fetched", string(payload))
	return payload, nil
}

func getPlainTextReader(body io.ReadCloser, encoding string) io.ReadCloser {
	switch encoding {
	case "gzip":
		reader, err := gzip.NewReader(body)
		if err != nil {
			log.Println("error decoding gzip response", reader)
			log.Println("will return raw body")
			return body
		}
		return reader
	case "br":
		reader := brotli.NewReader(body)
		if reader == nil {
			log.Println("error decoding br response", reader)
			log.Println("will return raw body")
			return body
		}
		return io.NopCloser(reader)
	default:
		return body
	}
}

func xor(a, b bool) bool {
	if a == b {
		return false
	} else {
		return true
	}
}
