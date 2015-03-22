package main

import (
	"bufio"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

var (
	addr = flag.String("a", ":9625", "Address to listen on")
)

var pageTmpl = template.Must(template.New("HTML").Parse(
	`<html>
	<head>
		<meta name="go-import" content="{{.Host}}{{.Path}} {{.Vcs}} {{.Url}}">
	</head>
	<body></body>
</html>`))

type Source struct {
	Vcs, Url string
}

type Router map[string]Source

type Server struct {
	config string
	Routes chan Router
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var match struct {
		Source
		Host string
		Path string
	}
	if r.Method != "GET" {
		http.Error(w, "Method Not Allowed", 405)
		return
	}
	routes := <-s.Routes

	if src, root, ok := routes.findPath(r.URL.Path); !ok {
		http.Error(w, "Not Found", 404)
		return
	} else {
		match.Source = src
		match.Host = r.Host
		match.Path = root
	}
	if r.FormValue("go-get") != "1" {
		// if this request is not coming from the go tool, redirect
		// to godoc.org
		http.Redirect(w, r, "http://godoc.org/"+r.Host+r.URL.Path, http.StatusSeeOther)
		return
	}
	if err := pageTmpl.Execute(w, match); err != nil {
		log.Print(err)
	}
}

func (r Router) findPath(path string) (Source, string, bool) {
	nodes := strings.Split(path, "/")
	for len(nodes) > 0 {
		path := strings.Join(nodes, "/")
		if src, ok := r[path]; ok {
			return src, path, true
		}
		nodes = nodes[:len(nodes)-1]
	}
	return Source{}, "", false
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("")
	flag.Usage = func() {
		log.Printf("Usage: %s [-a addr] config\n", os.Args[0])
		os.Exit(2)
	}
	flag.Parse()

	if len(flag.Args()) != 1 {
		flag.Usage()
	}

	s := NewServer(flag.Arg(0))
	srv := &http.Server{
		Handler: s,
		Addr:    *addr,
	}
	go srv.ListenAndServe()
	log.Print("Listening on ", *addr)
	if err := s.loadConfig(); err != nil {
		log.Fatal(err)
	}
}

func NewServer(config string) *Server {
	return &Server{
		config: config,
		Routes: make(chan Router),
	}
}

// runs in its own goroutine.
func (srv *Server) loadConfig() error {
	r, err := NewRouter(srv.config)
	if err != nil {
		return err
	}
	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGHUP)

	for {
		select {
		case srv.Routes <- r:
		case <-sig:
			if nr, err := NewRouter(srv.config); err != nil {
				log.Print(err)
			} else {
				r = nr
			}
		}
	}
}

func NewRouter(filename string) (Router, error) {
	r := make(Router)

	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	n := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		n++
		if strings.HasPrefix(scanner.Text(), "#") {
			continue
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 {
			continue
		}
		if len(fields) != 3 {
			continue
			log.Printf("%s:%d: (%d) fields (expected 3) in %q",
				filename, n, len(fields), scanner.Text())
		}
		if _, ok := r[fields[0]]; ok {
			return nil, fmt.Errorf("%s:%d: duplicate entry %s", filename, n, fields[0])
		}
		r[fields[0]] = Source{fields[1], fields[2]}
	}
	return r, scanner.Err()
}
