package main

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
)

func numbersHandler(wr http.ResponseWriter, req *http.Request) {
	if err := pages.ExecuteTemplate(wr, "numbers.html", mux.Vars(req)); err != nil {
		fmt.Fprintln(wr, err)
	}
}
