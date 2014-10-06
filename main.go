package main

import (
	"flag"
	"fmt"
	"github.com/SlyMarbo/spdy"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"
)

var (
	domain string
)

func handler(rw http.ResponseWriter, req *http.Request) {
	req.URL, _ = url.Parse(fmt.Sprintf("https://%s%s", domain, req.RequestURI))
	req.RequestURI = ""

	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("%4s %s: Error: %s", req.Method, req.URL.String(), err)
		return
	}

	for k, vs := range resp.Header {
		for _, v := range vs {
			rw.Header().Add(k, v)
		}
	}
	rw.WriteHeader(resp.StatusCode)
	body, _ := ioutil.ReadAll(resp.Body)
	rw.Write(body)
	log.Printf("%4s %s: %fms\n", req.Method, req.URL.String(), time.Now().Sub(start).Seconds()*1000)
}

func main() {
	flag.StringVar(&domain, "domain", "", "domain to proxy")
	bind := flag.String("bind", ":44300", "bind to")
	flag.Parse()
	http.HandleFunc("/", handler)
	log.Printf("Proxing to %s on %s", domain, *bind)
	err := spdy.ListenAndServeTLS(*bind, "server.crt", "server.key", nil)
	if err != nil {
		log.Fatal(err)
	}
}
