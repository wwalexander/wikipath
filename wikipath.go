package main

import (
	"encoding/json"
	"errors"
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
}

type APIQuery struct {
	Pages map[string]APIPage `json:"pages"`
}

type APIContinue struct {
	PLContinue string `json:"plcontinue"`
	Continue   string `json:"continue"`
}

type APIResp struct {
	Query    APIQuery     `json:"query"`
	Continue *APIContinue `json:"continue"`
}

func Get(url url.URL) (APIResp, error) {
	resp, err := http.Get(url.String())
	if err != nil {
		return APIResp{}, err
	}
	var ar APIResp
	if err = json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return APIResp{}, err
	}
	return ar, nil
}

func (ar APIResp) APIPage() (APIPage, bool) {
	for _, p := range ar.Query.Pages {
		if p.Missing == nil {
			return p, true
		}
	}
	return APIPage{}, false
}

func Links(title string) ([]*APILink, error) {
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
	links := []*APILink{}
	for {
		ar, err := Get(url)
		if err != nil {
			return nil, err
		}
		ap, ok := ar.APIPage()
		if !ok {
			return nil, errors.New("page does not exist")
		} else if ap.NS != 0 {
			return nil, errors.New("page is namespaced")
		}
		links = append(links, ap.Links...)
		if ar.Continue == nil {
			break
		}
		q.Set("continue", ar.Continue.Continue)
		q.Set("plcontinue", ar.Continue.PLContinue)
		url.RawQuery = q.Encode()
	}
	return links, nil
}

type Page struct {
	Title  string
	Parent *Page
}

type Path []string

func NewPath(t *Page) Path {
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
	queue := []*Page{&Page{Title: s}}
	for len(queue) > 0 {
		top := queue[0]
		queue = queue[1:]
		path := NewPath(top)
		if top.Title == t {
			return path, true
		}
		log.Println(path)
		links, err := Links(top.Title)
		if err != nil {
			continue
		}
		for _, l := range links {
			if _, ok := visited[l.Title]; ok {
				continue
			}
			p := &Page{
				Title:  l.Title,
				Parent: top,
			}
			queue = append(queue, p)
			visited[p.Title] = true
		}
	}
	return nil, false
}

func main() {
	if len(os.Args) < 3 {
		log.Fatal("start and end article must be specified")
	}
	path, ok := Walk(os.Args[1], os.Args[2])
	if !ok {
		log.Fatal("no path exists")
	}
	fmt.Println(path)
}
