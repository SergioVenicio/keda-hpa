package main

import (
	"encoding/json"
	"net/http"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		success := struct {
			Message string `json:"message"`
		}{
			Message: "success",
		}
		json.NewEncoder(w).Encode(&success)
	})
	if err := http.ListenAndServe("0.0.0.0:5000", mux); err != nil {
		panic(err)
	}
}
