package main

import (
	"flag"
	"fmt"
	"github.com/SlyMarbo/spdy"
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

	rw.WriteHeader(resp.StatusCode)
	for k, vs := range resp.Header {
		for _, v := range vs {
			rw.Header().Add(k, v)
		}
	}

	buf := make([]byte, 1024)
	for {
		n, err := resp.Body.Read(buf)
		if err != nil {
			resp.Body.Close()
			break
		}
		rw.Write(buf[:n])
	}
	log.Printf("%4s %s: %fms\n", req.Method, req.URL.String(), time.Now().Sub(start).Seconds()*1000)
}

func ping() {
	ping, err := spdy.PingServer(*http.DefaultClient, fmt.Sprintf("https://%s", domain))
	if err != nil {
		log.Printf("Error pinging: %s", err)
	} else {
		_, ok := <-ping
		if ok {
			log.Printf("Ping OK.")
		}
	}
}

func main() {
	flag.StringVar(&domain, "domain", "", "domain to proxy")
	bind := flag.String("bind", ":44300", "bind to")
	cert := flag.String("cert", "server.crt", "ssl certificate")
	key := flag.String("key", "server.key", "ssl key")
	flag.Parse()

	if len(domain) == 0 {
		log.Fatal("You must supply a domain with -domain=<domain>")
	}

	http.HandleFunc("/", handler)
	log.Printf("Proxing to %s on %s", domain, *bind)

	pingTicker := time.NewTicker(time.Second * 60)
	go func() {
		for _ = range pingTicker.C {
			ping()
		}
	}()

	err := spdy.ListenAndServeTLS(*bind, *cert, *key, nil)
	if err != nil {
		log.Fatal(err)
	}
}
