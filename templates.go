package main

// This file contains functions to do with rendering templates to a
// http.ResponseWriter.

import (
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/oxtoacart/bpool"
)

type M map[string]interface{}

var (
	templates map[string]*template.Template
	bufpool   *bpool.BufferPool

	// Template functions go here
	templateFuncs = template.FuncMap{}
)

func init() {
	templates = make(map[string]*template.Template)

	// Read the base template.
	baseb, err := Asset("base.tmpl")
	if err != nil {
		panic(err)
	}
	base := string(baseb)

	// Parse all other templates.
	for _, asset := range AssetNames() {
		ext := filepath.Ext(asset)

		if ext == ".tmpl" && asset != "base.tmpl" {
			name := asset[0 : len(asset)-5]
			data, _ := Asset(asset)

			// Mimic the ParseFiles function manually here
			t := template.New(name).Funcs(templateFuncs)
			template.Must(t.Parse(string(data)))
			template.Must(t.New("base").Parse(base))

			templates[name] = t
		}
	}

	bufpool = bpool.NewBufferPool(20)
}

// renderTemplate is a wrapper around template.ExecuteTemplate.
// It writes into a bytes.Buffer before writing to the http.ResponseWriter to catch
// any errors resulting from populating the template.
func renderTemplate(w http.ResponseWriter, name string, data M) error {
	// Ensure the template exists in the map.
	tmpl, ok := templates[name]
	if !ok {
		return fmt.Errorf("The template %s does not exist", name)
	}

	// Create a buffer to temporarily write to and check if any errors were encounted.
	buf := bufpool.Get()
	defer bufpool.Put(buf)

	err := tmpl.ExecuteTemplate(buf, "base", data)
	if err != nil {
		return err
	}

	// Set the header and write the buffer to the http.ResponseWriter
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
	return nil
}

func renderErrorTemplate(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Error data
	data := M{
		"message": message,
		"code":    code,
	}

	// Create a buffer to temporarily write to and check if any errors were encounted.
	buf := bufpool.Get()
	defer bufpool.Put(buf)

	err := templates["error"].ExecuteTemplate(buf, "base", data)
	if err != nil {
		http.Error(w, "error rendering error template o_0", 500)
		return
	}

	w.WriteHeader(code)
	buf.WriteTo(w)
}
