package main

import (
	"encoding/json"
	"net/http"
)

type errorInfo struct {
	err  error
	msg  string
	code int
}

func printError(w http.ResponseWriter, i errorInfo) {
	js := map[string]interface{}{
		"status": "error",
	}
	if len(i.msg) > 0 {
		js["msg"] = i.msg
	}
	if i.err != nil {
		js["error"] = i.err.Error()
	}

	if i.code == 0 {
		i.code = 500
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(i.code)
	json.NewEncoder(w).Encode(js)
}

func printJson(w http.ResponseWriter, js map[string]interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(js)
}
