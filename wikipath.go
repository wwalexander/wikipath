package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
)

type page struct {
	title  string
	parent *page
	links  []*page
}

type apiPage struct {
	Title   string  `json:"title"`
	NS      int     `json:"ns"`
	Missing *string `json:"missing"`
	Links   []*page `json:"links"`
	Parent  *page   `json:"-"`
}

type apiQuery struct {
	Pages map[string]*apiPage `json:"pages"`
}

type apiContinue struct {
	PLContinue string `json:"plcontinue"`
	Continue   string `json:"continue"`
}

type apiResp struct {
	Query    apiQuery     `json:"query"`
	Continue *apiContinue `json:"continue"`
}

func (ar apiResp) apiPage() (*apiPage, error) {
	for _, p := range ar.Query.pages {
		if p.Missing == nil && p.NS == 0 {
			return p, nil
		}
	}
	return nil, errors.New("non-namespaced page does not exist")
}

func baseURL(title string) (*url.URL, url.Values) {
	url := url.URL{
		Scheme: "https",
		Host:   "en.wikipedia.org",
		Path:   path.Join("w", "api.php"),
	}
	q := url.Query()
	q.Set("action", "query")
	q.Set("format", "json")
	q.Set("titles", title)
	return &url, q
}

func newPage(title string) (*page, error) {
	url, q := baseURL(title)
	url.RawQuery = q.Encode()
	resp, err := http.Get(url.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	ar := apiResp{}
	if err = json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return nil, err
	}
	ap, err := ar.apiPage()
	if err != nil {
		return nil, err
	}
	return &page{title: ap.Title}, nil
}

func (p *page) populateLinks() error {
	url, q := baseURL(p.title)
	q.Set("prop", "links")
	url.RawQuery = q.Encode()
	for {
		url.RawQuery = q.Encode()
		resp, err := http.Get(url.String())
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		var ar apiResp
		if err = json.NewDecoder(resp.Body).Decode(&ar); err != nil {
			return err
		}
		ap, err := ar.apiPage()
		if err != nil {
			return err
		}
		p.links = append(p.links, ap.Links...)
		if ar.Continue == nil {
			return nil
		}
		q.Set("continue", ar.Continue.Continue)
		q.Set("plcontinue", ar.Continue.PLContinue)
	}
}

// Path is the list of page titles between two pages (inclusive).
type Path []string

func newPath(t *page) Path {
	path := Path{}
	for p := t; p != nil; p = p.parent {
		path = append(Path{p.title}, path...)
	}
	return path
}

// String returns the titles in path delimited with an arrow.
func (p Path) String() string {
	return strings.Join(p, " -> ")
}

// Walk returns the shortest path between the pages with titles s and t.
func Walk(s string, t string) (p Path, err error) {
	visited := make(map[string]bool)
	sp, err := newPage(s)
	if err != nil {
		return nil, err
	}
	if s == t {
		return newPath(sp), nil
	}
	if err = sp.populateLinks(); err != nil {
		return nil, err
	}
	queue := []*page{sp}
	for len(queue) > 0 {
		top := queue[0]
		queue = queue[1:]
		log.Println(top.title)
		pages := make(chan *page, len(top.links))
		for _, l := range top.links {
			go func(p *page) {
				if _, ok := visited[p.title]; ok {
					pages <- nil
					return
				}
				p.parent = top
				if p.title == t {
					pages <- p
					return
				}
				if err = p.populateLinks(); err != nil {
					pages <- nil
					return
				}
				pages <- p
			}(l)
		}
		pagesOut := make([]*page, 0, len(top.links))
		for range top.links {
			p := <-pages
			if p == nil {
				continue
			}
			if p.title == t {
				return newPath(p), nil
			}
			pagesOut = append(pagesOut, p)
		}
		close(pages)
		for _, p := range pagesOut {
			visited[p.title] = true
			queue = append(queue, p)
		}
	}
	return nil, errors.New("no path exists")
}

const usage = `usage: wikipath [title] [title]

wikipath finds the shortest path from the Wikipedia article specified by the
first title to the Wikipedia article specified by the second title.`

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) != 2 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(1)
	}
	s := args[0]
	t := args[1]
	path, err := Walk(s, t)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(path)
}
