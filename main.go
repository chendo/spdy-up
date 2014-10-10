package main

import (
	"compress/gzip"
	"errors"
	"flag"
	"fmt"
	"github.com/SlyMarbo/spdy"
	"io"
	"log"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"
)

type Proxy struct {
	Domain     string
	OriginHost string
}

var (
	proxies     map[string]*Proxy
	client      *http.Client
	secret      string
	redirectErr error
)

func init() {
	redirectErr = errors.New("Redirect")
	runtime.GOMAXPROCS(runtime.NumCPU())
}

func handler(rw http.ResponseWriter, req *http.Request) {
	proxy, ok := proxies[req.Host]

	s := strings.SplitN(req.RemoteAddr, ":", 2)
	remoteIP := s[0]

	var (
		isSpdyClient bool
		info         string
		requestURL   string
	)
	if req.URL.Host != "" { // SPDY requests come through with Host
		isSpdyClient = true
		requestURL = req.URL.String()
	} else {
		isSpdyClient = false
		var scheme string
		if req.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
		requestURL = scheme + "://" + req.Host + req.RequestURI
	}
	if isSpdyClient {
		info = "S"
	} else {
		info = " "
	}

	if !ok {
		// No proxy mapping
		rw.WriteHeader(400)
		rw.Write([]byte("Bad Request\n"))
		log.Printf("%15s %s[---] %5s %s: Error: Invalid domain", remoteIP, info, req.Method, requestURL)
		return
	}

	req.URL, _ = url.Parse(fmt.Sprintf("https://%s%s", proxy.OriginHost, req.RequestURI))
	req.RequestURI = "" // http.Client requests cannot have RequestURI
	req.Host = proxy.Domain
	req.Header["X-Forwarded-For"] = []string{remoteIP}
	req.Header["SPDY-Up-Connecting-IP"] = []string{remoteIP}
	req.Header["SPDY-Up-Secret"] = []string{secret}

	start := time.Now()
	var (
		err  error
		resp *http.Response
	)
	for tries := 0; tries < 2; tries++ {
		resp, err = client.Do(req)
		if err != nil {
			urlErr, ok := err.(*url.Error)
			if ok && urlErr.Err == redirectErr {
				// redirect should go pass through
				err = nil
				break
			}
			log.Printf("%15s %s[---] %5s %s: Error: %+v", remoteIP, info, req.Method, requestURL, urlErr)
		} else {
			break
		}
	}

	if err != nil {
		rw.WriteHeader(502)
		rw.Write([]byte("Could not reach origin\n"))
		return
	}

	for k, vs := range resp.Header {
		if k[0:1] == ":" {
			continue
		}
		for _, v := range vs {
			rw.Header().Add(k, v)
		}
	}
	body := resp.Body
	defer resp.Body.Close()
	// response comes back as gzip even though client never requested it
	if strings.Join(resp.Header["Content-Encoding"], "") == "gzip" && !strings.Contains(strings.Join(req.Header["Accept-Encoding"], " "), "gzip") {
		body, err = gzip.NewReader(body)
		resp.Header["Content-Encoding"] = []string{}
		defer body.Close()
	}
	rw.WriteHeader(resp.StatusCode)

	io.Copy(rw, body)
	log.Printf("%15s %s[%d] %5s %s: %.3fms\n", remoteIP, info, resp.StatusCode, req.Method, requestURL, time.Now().Sub(start).Seconds()*1000)
}

func ping() {
	for domain, proxy := range proxies {
		ping, err := spdy.PingServer(*client, fmt.Sprintf("https://%s", proxy.OriginHost))
		if err != nil {
			log.Printf("Error pinging %s: %s", domain, err)
		} else {
			_, ok := <-ping
			if !ok {
				log.Printf("Error pinging %s: %s", domain, err)
			}
		}

	}
}

func healthcheck(rw http.ResponseWriter, req *http.Request) {
	rw.Write([]byte("ok"))
}

func main() {
	bind := flag.String("bind", ":8000", "bind address for http")
	sslbind := flag.String("sslbind", ":44300", "bind address for https")
	cert := flag.String("cert", "server.crt", "ssl certificate")
	key := flag.String("key", "server.key", "ssl key")
	keepalive := flag.Bool("keepalive", true, "use pings to keep the spdy clients alive")
	noSpdyClient := flag.Bool("no-spdy-client", false, "disable spdy client")
	flag.StringVar(&secret, "secret", "secret", "secret token sent to origin to authenticate source IP")
	flag.Parse()

	transport := spdy.NewTransport(false)
	transport.ResponseHeaderTimeout = time.Second * 5
	transport.DisableCompression = true // does nothing, pretty sure it's a bug

	client = &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return redirectErr
		},
	}

	if *noSpdyClient {
		client.Transport = nil
	}

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

	http.HandleFunc("/__healthcheck", healthcheck)
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
			srv := http.Server{
				Addr:         *bind,
				ReadTimeout:  time.Second * 5,
				WriteTimeout: time.Second * 5,
			}
			err := srv.ListenAndServe()
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
