package main

import (
	"flag"
	"fmt"
	"github.com/SlyMarbo/spdy"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Proxy struct {
	Domain     string
	OriginHost string
	Client     *http.Client
}

var (
	proxies map[string]*Proxy
)

func handler(rw http.ResponseWriter, req *http.Request) {
	proxy, ok := proxies[req.Host]
	if !ok {
		// No proxy mapping
		rw.WriteHeader(400)
		rw.Write([]byte("Bad Request"))
		return
	}

	domain := proxy.Domain
	originalURL := req.URL

	req.URL, _ = url.Parse(fmt.Sprintf("https://%s%s", proxy.OriginHost, req.RequestURI))
	req.RequestURI = "" // http.Client requests cannot have RequestURI
	req.Host = proxy.Domain

	start := time.Now()
	var (
		err  error
		resp *http.Response
	)
	for tries := 0; tries < 2; tries++ {
		client := proxy.Client
		if client == nil {
			client = spdy.NewClient(false)
			proxy.Client = client
		}
		resp, err = client.Do(req)
		if err != nil {
			log.Printf("%5s %s%s: Error: %s", req.Method, domain, originalURL.String(), err)
			proxy.Client = nil
		} else {
			break
		}
	}

	if err != nil {
		rw.WriteHeader(500)
		rw.Write([]byte("Could not reach origin"))
		return
	}

	defer resp.Body.Close()

	rw.WriteHeader(resp.StatusCode)
	for k, vs := range resp.Header {
		for _, v := range vs {
			rw.Header().Add(k, v)
		}
	}

	io.Copy(rw, resp.Body)
	log.Printf("%5s %s%s: %.3fms\n", req.Method, domain, originalURL.String(), time.Now().Sub(start).Seconds()*1000)
}

func ping() {
	for domain, proxy := range proxies {
		if proxy.Client == nil {
			continue
		}
		ping, err := spdy.PingServer(*proxy.Client, fmt.Sprintf("https://%s", proxy.OriginHost))
		if err != nil {
			log.Printf("Error pinging %s: %s", domain, err)
			proxy.Client = nil
		} else {
			_, ok := <-ping
			if !ok {
				log.Printf("Error pinging %s: %s", domain, err)
				proxy.Client = nil
			}
		}

	}
}

func main() {
	bind := flag.String("bind", ":8000", "bind address for http")
	sslbind := flag.String("sslbind", ":44300", "bind address for https")
	cert := flag.String("cert", "server.crt", "ssl certificate")
	key := flag.String("key", "server.key", "ssl key")
	keepalive := flag.Bool("keepalive", true, "use pings to keep the spdy clients alive")
	flag.Parse()

	proxies = make(map[string]*Proxy)

	for _, domainDefinition := range flag.Args() {
		components := strings.SplitN(domainDefinition, ":", 2)
		domain := components[0]
		origin := components[1]
		proxies[domain] = &Proxy{
			Domain:     domain,
			OriginHost: origin,
		}
		log.Printf("Proxing to %s to %s", domain, origin)
	}

	if len(proxies) == 0 {
		log.Fatal("You must supply at least one definition with domain.com:origin-ip")
	}

	http.HandleFunc("/", handler)

	if *keepalive {
		pingTicker := time.NewTicker(time.Second * 60)
		go func() {
			for _ = range pingTicker.C {
				ping()
			}
		}()
	}
	if *bind != "" {
		log.Printf("Listening to HTTP on %s", *bind)
		go func() {
			err := http.ListenAndServe(*bind, nil)
			if err != nil {
				log.Fatal(err)
			}
		}()
	}

	if *sslbind != "" {
		log.Printf("Listening to HTTPS on %s", *sslbind)
		go func() {
			err := spdy.ListenAndServeTLS(*sslbind, *cert, *key, nil)
			if err != nil {
				log.Fatal(err)
			}
		}()
	}
	select {}
}
