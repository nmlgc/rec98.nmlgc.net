package main

import (
	"fmt"
	"net/http"
)

func indexHandler(wr http.ResponseWriter, req *http.Request) {
	if err := pages.ExecuteTemplate(wr, "index.html", nil); err != nil {
		fmt.Fprintln(wr, err)
	}
}
