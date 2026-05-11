package utils

import (
	"bytes"
	"io"
	"log"
	"net/http"
)

type RoundTripper struct{}

func (r RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	body, _ := io.ReadAll(resp.Body)

	log.Println("========== MATRIX RESPONSE ==========")
	log.Println("STATUS:", resp.Status)
	log.Println("BODY:", string(body))
	log.Println("=====================================")

	resp.Body = io.NopCloser(bytes.NewBuffer(body))

	return resp, nil
}
