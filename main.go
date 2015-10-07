package main

import (
	"flag"
	"io/ioutil"
	"log"

	"github.com/shawnsi/cacheproxy/proxy"
	"github.com/shawnsi/cacheproxy/router"
)

var managerPort = flag.String("manager", "9190", "manager port")
var proxyPort = flag.String("proxy", "9080", "proxy port")
var routerPort = flag.String("router", "9090", "router port")
var replicas = flag.Int("replicas", 3, "cache replica count")
var verbose = flag.Bool("v", false, "verbose mode")

func main() {
	flag.Parse()
	proxy := proxy.New(replicas)
	router := router.New()

	// Only log to console if verbose flag is used
	if !*verbose {
		log.SetOutput(ioutil.Discard)
	}

	// Pass all remaining arguments in as backend
	backends := flag.Args()
	for index := range backends {
		log.Println("Adding backend: " + backends[index])
		proxy.Add(backends[index])
	}

	// Initialize and run router in background via goroutine
	log.Println("Starting router on port: " + *routerPort)
	go router.Serve(routerPort)

	// Initialize and run proxy in background via goroutine
	log.Println("Starting proxy on port: " + *proxyPort)
	go proxy.Serve(proxyPort, routerPort)

	log.Println("Starting proxy manager on port: " + *managerPort)
	proxy.Manage(managerPort)
}
