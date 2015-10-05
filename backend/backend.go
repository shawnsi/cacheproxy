package main

import (
	"io"
	"math/rand"
	"net/http"
	"os"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		values := []string{
			"HIT",
			"MISS",
		}

		w.Header().Set("X-Cache", values[rand.Intn(len(values))])

		io.WriteString(w, req.Header.Get("X-Backends")+"\n")
	})
	http.ListenAndServe(":"+os.Args[1], nil)
}
