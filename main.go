package main

import (
	"fmt"
	"github.com/SlyMarbo/spdy"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func handler(rw http.ResponseWriter, req *http.Request) {
	req.Host = strings.Replace(req.Host, "au.", "", 1)
	req.Host = strings.Replace(req.Host, ":44300", "", 1)
	req.URL, _ = url.Parse(fmt.Sprintf("https://%s%s", req.Host, req.RequestURI))
	req.RequestURI = ""

	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
	}

	for k, vs := range resp.Header {
		for _, v := range vs {
			rw.Header().Add(k, v)
		}
	}
	rw.WriteHeader(resp.StatusCode)
	body, _ := ioutil.ReadAll(resp.Body)
	rw.Write(body)
	fmt.Printf("%4s %s: %f\n", req.Method, req.URL.String(), time.Now().Sub(start).Seconds())
}

func main() {
	http.HandleFunc("/", handler)
	err := spdy.ListenAndServeTLS(":44300", "server.crt", "server.key", nil)
	if err != nil {
		log.Fatal(err)
	}
}
