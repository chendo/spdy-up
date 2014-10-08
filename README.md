# SPDY-UP

SPDY-UP speeds up API requests made by stateless clients (no keepalive/SPDY) to SPDY-enabled servers by keeping a SPDY connection open on behalf of clients.

It's kind of like Cloudflare's [Railgun](https://www.cloudflare.com/railgun) in a way, but it's usable on their Free plan.

It's most effective with HTTPS.

Probably not production ready.

## Performance

Setup:
* Origin is on Heroku (US East)
* Cloudflare-enabled
* SPDY-UP on a VM in Australia
* Route 53 Geolocation DNS serves up Cloudflare by default, but Australian node for Oceania

Tested on a residential connection in Australia.

### Without SPDY-UP

```
# origin.slim.cat always resolves to Cloudflare
$ time curl -s https://origin.slim.cat -H "Host: slim.cat" > /dev/null
1.509s total, 0.33s user, 0.02s system, 22% cpu
```

### With SPDY-UP

```
$ time curl -s https://slim.cat > /dev/null                                                                                                                                     ◼ 22:38:35
0.920s total, 0.05s user, 0.01s system, 6% cpu
```

## License

MIT
