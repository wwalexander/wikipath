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

// APILink represents an link in an API response.
type APILink struct {
	Title string `json:"title"`
}

// APIPage represents a page in an API response.
type APIPage struct {
	Title   string     `json:"title"`
	NS      int        `json:"ns"`
	Missing *string    `json:"missing"`
	Links   []*APILink `json:"links"`
	Parent  *APIPage   `json:"-"`
}

// APIQuery represents a query response in an API response.
type APIQuery struct {
	Pages map[string]*APIPage `json:"pages"`
}

// APIContinue represents a pagination continue value in an API response.
type APIContinue struct {
	PLContinue string `json:"plcontinue"`
	Continue   string `json:"continue"`
}

// APIResp represents an API response.
type APIResp struct {
	Query    APIQuery     `json:"query"`
	Continue *APIContinue `json:"continue"`
}

var baseURL url.URL
var baseQ url.Values

func init() {
	baseURL = url.URL{
		Scheme: "https",
		Host:   "en.wikipedia.org",
		Path:   path.Join("w", "api.php"),
	}
	baseQ = baseURL.Query()
	baseQ.Set("action", "query")
	baseQ.Set("format", "json")
	baseQ.Set("prop", "links")
}

// GetAPIResp gets an APIResp for the given title. If prev.Continue is not nil,
// its values are used to continue pagination.
func GetAPIResp(title string, prev *APIResp) (*APIResp, error) {
	q := baseQ
	if prev.Continue != nil {
		q.Set("continue", prev.Continue.Continue)
		q.Set("plcontinue", prev.Continue.PLContinue)
	}
	q.Set("titles", title)
	url := baseURL
	url.RawQuery = q.Encode()
	resp, err := http.Get(url.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	ar := &APIResp{}
	if err = json.NewDecoder(resp.Body).Decode(ar); err != nil {
		return nil, err
	}
	return ar, nil
}

// GetAPIPage gets an APIPage for the given title.
func GetAPIPage(title string) (*APIPage, error) {
	ap := &APIPage{
		Title: title,
		Links: []*APILink{},
	}
	prev := &APIResp{}
	for {
		ar, err := GetAPIResp(title, prev)
		if err != nil {
			return nil, err
		}
		var apPart *APIPage
		for _, p := range ar.Query.Pages {
			apPart = p
		}
		if apPart == nil {
			return nil, errors.New("page does not exist")
		}
		ap.Links = append(ap.Links, apPart.Links...)
		prev = ar
		if ar.Continue == nil {
			break
		}
	}
	return ap, nil
}

// Path is the list of page titles between two pages (inclusive).
type Path []string

// NewPath creates a new Path by tracing t's Parents.
func NewPath(t *APIPage) Path {
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

// Valid returns whether ap exists and is non-namespaced
func (ap APIPage) Valid() bool {
	return ap.Missing == nil && ap.NS == 0
}

// Walk returns the shortest path between the pages with titles s and t. If no
// path exists, ok is false.
func Walk(s string, t string) (p Path, ok bool) {
	visited := make(map[string]bool)
	sPage, err := GetAPIPage(s)
	if err != nil || !sPage.Valid() {
		return nil, false
	}
	if s == t {
		return NewPath(&APIPage{Title: s}), true
	}
	queue := []*APIPage{sPage}
	for len(queue) > 0 {
		top := queue[0]
		queue = queue[1:]
		if !top.Valid() {
			continue
		}
		log.Println(top.Title)
		pages := make(chan *APIPage, len(top.Links))
		for _, l := range top.Links {
			go func(title string) {
				if _, ok := visited[title]; ok {
					pages <- nil
					return
				}
				ap, err := GetAPIPage(title)
				if err != nil {
					pages <- nil
					return
				}
				ap.Parent = top
				pages <- ap
			}(l.Title)
		}
		for range top.Links {
			ap := <-pages
			if ap == nil {
				continue
			}
			if ap.Title == t {
				return NewPath(ap), true
			}
			visited[ap.Title] = true
			queue = append(queue, ap)
		}
	}
	return nil, false
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
	path, ok := Walk(s, t)
	if !ok {
		log.Fatalf("no path exists between %s and %s", s, t)
	}
	fmt.Println(path)
}
