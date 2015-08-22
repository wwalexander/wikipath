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

var baseURL url.URL
var baseQ url.Values
var baseQLinks url.Values

func init() {
	baseURL = url.URL{
		Scheme: "https",
		Host:   "en.wikipedia.org",
		Path:   path.Join("w", "api.php"),
	}
	baseQ = baseURL.Query()
	baseQ.Set("action", "query")
	baseQ.Set("format", "json")
	baseQLinks = baseQ
	baseQLinks.Set("prop", "links")
}

// Page represents a Wikipedia page.
type Page struct {
	Title  string
	Parent *Page
	Links  []*Page
}

type apiPage struct {
	Title   string  `json:"title"`
	NS      int     `json:"ns"`
	Missing *string `json:"missing"`
	Links   []*Page `json:"links"`
	Parent  *Page   `json:"-"`
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
	for _, p := range ar.Query.Pages {
		if p.Missing == nil && p.NS == 0 {
			return p, nil
		}
	}
	return nil, errors.New("page does not exist")
}

// GetPage gets a Page with the given title, without Links.
func GetPage(title string) (*Page, error) {
	q := baseQ
	q.Set("titles", title)
	url := baseURL
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
	return &Page{Title: ap.Title}, nil
}

// PopulateLinks populates p.Links.
func (p *Page) PopulateLinks() error {
	q := baseQLinks
	q.Set("titles", p.Title)
	url := baseURL
	url.RawQuery = q.Encode()
	resp, err := http.Get(url.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	ar := apiResp{}
	if err = json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return err
	}
	for ar.Continue != nil {
		ap, err := ar.apiPage()
		if err != nil {
			return err
		}
		p.Links = append(p.Links, ap.Links...)
		q.Set("continue", ar.Continue.Continue)
		q.Set("plcontinue", ar.Continue.PLContinue)
		url.RawQuery = q.Encode()
		resp, err := http.Get(url.String())
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		ar = apiResp{}
		if err = json.NewDecoder(resp.Body).Decode(&ar); err != nil {
			return err
		}
	}
	return nil
}

// Path is the list of page titles between two pages (inclusive).
type Path []string

// NewPath creates a new Path by tracing t's Parents.
func NewPath(t *Page) Path {
	path := Path{}
	for p := t; p != nil; p = p.Parent {
		path = append(Path{p.Title}, path...)
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
	sp, err := GetPage(s)
	if err != nil {
		return nil, err
	}
	if s == t {
		return NewPath(sp), nil
	}
	if err = sp.PopulateLinks(); err != nil {
		return nil, err
	}
	queue := []*Page{sp}
	for len(queue) > 0 {
		top := queue[0]
		queue = queue[1:]
		log.Println(top.Title)
		pages := make(chan *Page, len(top.Links))
		for _, l := range top.Links {
			go func(p *Page) {
				if _, ok := visited[p.Title]; ok {
					pages <- nil
					return
				}
				p.Parent = top
				if p.Title != t {
					if err = p.PopulateLinks(); err != nil {
						pages <- nil
						return
					}
				}
				pages <- p
			}(l)
		}
		for range top.Links {
			p := <-pages
			if p == nil {
				continue
			}
			if p.Title == t {
				return NewPath(p), nil
			}
			visited[p.Title] = true
			queue = append(queue, p)
		}
	}
	return nil, errors.New("no path exists")
}

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) != 2 {
		flag.Usage()
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
