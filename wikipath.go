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

type APILink struct {
	Title string `json:"title"`
}

type APIPage struct {
	Title   string     `json:"title"`
	NS      int        `json:"ns"`
	Missing *string    `json:"missing"`
	Links   []*APILink `json:"links"`
	Parent  *APIPage   `json:"-"`
}

type APIQuery struct {
	Pages map[string]*APIPage `json:"pages"`
}

type APIContinue struct {
	PLContinue string `json:"plcontinue"`
	Continue   string `json:"continue"`
}

type APIResp struct {
	Query    APIQuery     `json:"query"`
	Continue *APIContinue `json:"continue"`
}

func Get(url url.URL) (*APIResp, error) {
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

func GetAPIPage(title string) (*APIPage, error) {
	url := url.URL{
		Scheme: "https",
		Host:   "en.wikipedia.org",
		Path:   path.Join("w", "api.php"),
	}
	q := url.Query()
	q.Set("action", "query")
	q.Set("continue", "")
	q.Set("format", "json")
	q.Set("prop", "links")
	q.Set("titles", title)
	url.RawQuery = q.Encode()
	ap := &APIPage{
		Title: title,
		Links: []*APILink{},
	}
	for {
		ar, err := Get(url)
		if err != nil {
			return nil, err
		}
		var apPart *APIPage
		for _, p := range ar.Query.Pages {
			if p.Missing == nil {
				apPart = p
			}
		}
		if apPart == nil {
			return nil, errors.New("page does not exist")
		}
		if apPart.NS != 0 {
			return nil, errors.New("page is namespaced")
		}
		ap.Links = append(ap.Links, apPart.Links...)
		if ar.Continue == nil {
			break
		}
		q.Set("continue", ar.Continue.Continue)
		q.Set("plcontinue", ar.Continue.PLContinue)
		url.RawQuery = q.Encode()
	}
	return ap, nil
}

type Path []string

func NewPath(t *APIPage) Path {
	path := Path{}
	for p := t; p != nil; p = p.Parent {
		path = append(Path{p.Title}, path...)
	}
	return path
}

func (p Path) String() string {
	return strings.Join(p, " -> ")
}

func Walk(s string, t string) (Path, bool) {
	visited := make(map[string]bool)
	sPage, err := GetAPIPage(s)
	if err != nil {
		return nil, false
	}
	queue := []*APIPage{sPage}
	for len(queue) > 0 {
		top := queue[0]
		queue = queue[1:]
		path := NewPath(top)
		if top.Title == t {
			return path, true
		}
		log.Println(path)
		pages := make(chan *APIPage, len(top.Links))
		for _, l := range top.Links {
			go func(title string) {
				if _, ok := visited[l.Title]; ok {
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
		for _ = range top.Links {
			ap := <-pages
			if ap == nil {
				continue
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
