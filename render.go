package main

import (
	"encoding/json"
	"net/http"

	"github.com/Sirupsen/logrus"
)

var (
	badJson = []byte(`{"status":"error","error":"error marshalling to json",` +
		`"meta":"internal error while marshalling to json"}`)
)

func renderJSON(w http.ResponseWriter, code int, data interface{}) {
	var result []byte
	var err error

	result, err = json.Marshal(data)
	if err != nil {
		log.WithField("err", err).Errorf("Error marshalling to JSON")
		result = badJson
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_, err = w.Write(result)

	// Too late to do anything else - we've already written the HTTP headers.
	if err != nil {
		log.WithField("err", err).Errorf("Error writing request")
	}
}

func renderError(w http.ResponseWriter, code int, err string, meta interface{}) {
	log.WithFields(logrus.Fields{
		"code":  code,
		"error": err,
		"meta":  meta,
	}).Errorf("Error occured while processing request")

	msg := map[string]interface{}{
		"status": "error",
		"error":  err,
	}
	if meta != nil {
		msg["meta"] = meta
	}

	renderJSON(w, code, msg)
}
