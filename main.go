package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	base := os.Getenv("INFRAI_BASE_URL")
	if base == "" {
		base = defaultBaseURL
	}
	seconds := 60
	if raw := os.Getenv("REDRIVE_INTERVAL_SECONDS"); raw != "" {
		var err error
		seconds, err = strconv.Atoi(raw)
		if err != nil || seconds < 1 {
			log.Fatal("REDRIVE_INTERVAL_SECONDS must be positive")
		}
	}
	client := &infraiClient{baseURL: base, key: key, http: &http.Client{Timeout: 15 * time.Second}}
	log.Fatal(http.ListenAndServe("127.0.0.1:8080", routes(client, newLedger(time.Duration(seconds)*time.Second))))
}
