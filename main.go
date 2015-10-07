package main

import (
	"flag"
	"io/ioutil"
	"log"
	"net/http"
)

var managerPort = flag.String("m", "9190", "manager port")
var proxyPort = flag.String("p", "9090", "proxy port")
var replicas = flag.Int("r", 3, "cache replica count")
var verbose = flag.Bool("v", false, "verbose mode")

func main() {
	flag.Parse()
	proxy := New()

	// Only log to console if verbose flag is used
	if !*verbose {
		log.SetOutput(ioutil.Discard)
	}

	// Pass all remaining arguments in as backend
	backends := flag.Args()
	for index := range backends {
		log.Println("Adding backend: " + backends[index])
		proxy.backends.Add(backends[index])
	}

	// Initialize and run manager server in background via goroutine
	log.Println("Starting proxy manager service on port: " + *managerPort)
	go http.ListenAndServe(":"+*managerPort, proxy.manager)

	log.Println("Starting proxy service on port: " + *proxyPort)
	http.ListenAndServe(":9090", proxy.proxy)
}
