package auth

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

var landing = template.Must(template.New("landing").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{if .Failed}}Sign-in failed{{else}}Signed in{{end}}</title>
<style>
body{font-family:system-ui,sans-serif;display:flex;min-height:100vh;margin:0;align-items:center;justify-content:center;background:#faf8f5;color:#2b2823}
main{max-width:28rem;padding:2rem;text-align:center}
h1{font-size:1.4rem;font-weight:600}
p{color:#6b655c;line-height:1.5}
@media(prefers-color-scheme:dark){body{background:#1c1a17;color:#ece8e1}p{color:#a39c90}}
</style>
</head>
<body>
<main>
{{if .Failed}}
<h1>Sign-in failed</h1>
<p>{{.Code}}{{if .Description}}: {{.Description}}{{end}}</p>
<p>Close this tab and try again.</p>
{{else}}
<h1>You are signed in{{if .Subject}} to {{.Subject}}{{end}}</h1>
<p>You can close this tab and return to the app.</p>
{{end}}
</main>
</body>
</html>
`))

type landingData struct {
	Subject     string
	Code        string
	Description string
	Failed      bool
}

// Loopback receives an OAuth redirect on the loopback interface.
type Loopback struct {
	listener net.Listener
	server   *http.Server
	host     string
	port     int
	path     string
	results  chan url.Values

	once sync.Once
}

// ListenLoopback starts a listener on 127.0.0.1 on the first free port of
// ports. A zero port, or no ports, lets the system choose one. host is the
// name used in the redirect URI, and subject is shown on the landing page.
func ListenLoopback(host string, ports []int, path, subject string) (*Loopback, error) {
	if len(ports) == 0 {
		ports = []int{0}
	}
	var (
		listener net.Listener
		err      error
	)
	for _, port := range ports {
		listener, err = (&net.ListenConfig{}).Listen(context.Background(), "tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("open sign-in callback port: %w", err)
	}

	l := &Loopback{
		listener: listener,
		host:     host,
		port:     listener.Addr().(*net.TCPAddr).Port,
		path:     path,
		results:  make(chan url.Values, 1),
	}
	mux := http.NewServeMux()
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		data := landingData{Subject: subject, Code: q.Get("error"), Description: q.Get("error_description")}
		data.Failed = data.Code != ""
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if data.Failed {
			w.WriteHeader(http.StatusBadRequest)
		}
		_ = landing.Execute(w, data)
		select {
		case l.results <- q:
		default:
		}
	})
	l.server = &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = l.server.Serve(listener) }()
	return l, nil
}

// RedirectURI returns the URI to register as the redirect for the sign-in.
func (l *Loopback) RedirectURI() string {
	return fmt.Sprintf("http://%s:%d%s", l.host, l.port, l.path)
}

// Results delivers the query of the redirect once the browser comes back.
func (l *Loopback) Results() <-chan url.Values { return l.results }

// Close stops the listener. It is safe to call more than once.
func (l *Loopback) Close() error {
	var err error
	l.once.Do(func() {
		if cerr := l.server.Close(); cerr != nil && !errors.Is(cerr, http.ErrServerClosed) {
			err = cerr
		}
	})
	return err
}
