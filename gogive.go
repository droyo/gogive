package main

import (
	"bufio"
	"flag"
	"log"
	"net/http"
	"os"
	"io"
	"os/signal"
	"fmt"
	"strings"
	"syscall"
	"html/template"
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

type Router map[string] Source

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
	routes := <- s.Routes
	
	if src, root, ok := routes.findPath(r.URL.Path); !ok {
		http.Error(w, "Not Found", 404)
		return
	} else {
		match.Source = src
		match.Host = r.Host
		match.Path = root
	}
	
	if err := pageTmpl.Execute(w, match); err != nil {
		http.Error(w, "Internal Server Error", 500)
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

func init() {
	log.SetFlags(0)
	log.SetPrefix("")
	flag.Usage = func() {
		log.Printf("Usage: %s [-a addr] config\n", os.Args[0])
		os.Exit(2)
	}
}

func main() {
	flag.Parse()

	if len(flag.Args()) != 1 {
		flag.Usage()
	}

	s := NewServer(flag.Arg(0))
	srv := &http.Server {
		Handler: s,
		Addr: *addr,
	}
	go srv.ListenAndServe()
	log.Print("Listening on ", *addr)
	if err := s.Run(); err != nil {
		log.Fatal(err)
	}
}

func NewServer(config string) *Server {
	return &Server {
		config: config,
		Routes: make(chan Router),
	}
}

func (srv *Server) Run() error {
	r, err := NewRouter(srv.config)
	if err != nil {
		return err
	}
	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGHUP, os.Interrupt, os.Kill)
	
	for {
		select {
		case srv.Routes <- r:
		case s := <-sig:
			if s == os.Interrupt || s == os.Kill {
				return fmt.Errorf("Interrupt")
			}
			nr, err := NewRouter(srv.config)
			if err != nil {
				log.Print(err)
			}
			r = nr
		}
	}
}

func NewRouter(filename string) (Router, error) {
	var n int
	r := make(Router)
	
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	buf := bufio.NewReader(file)
	for {
		line, err := buf.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		n++
		if len(line) < 2 {
			continue
		}
		if line[0] == '#' {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return nil, fmt.Errorf("%s:%d: too many fields in `%s'", filename, n, line)
			continue
		}
		if _, ok := r[fields[0]]; ok {
			return nil, fmt.Errorf("%s:%d: duplicate entry %s", filename, n, fields[0])
		}
		r[fields[0]] = Source{fields[1], fields[2]}
	}
	return r, nil
}
