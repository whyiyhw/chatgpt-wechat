package dify

import (
	"net/http"
	"time"
)

type ClientConfig struct {
	Host             string
	DefaultAPISecret string
	Timeout          time.Duration
	Transport        *http.Transport
}
