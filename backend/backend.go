package main

import (
	"io"
	"net/http"
	"os"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		io.WriteString(w, req.Header.Get("X-Backends")+"\n")

	})
	http.ListenAndServe(":"+os.Args[1], nil)
}
