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

func Walk(s string, t string) (Path, error) {
	visited := make(map[string]bool)
	queue := []*Page{&Page{Title: s}}
	for len(queue) > 0 {
		top := queue[0]
		queue = queue[1:]
		links, err := Links(top.Title)
		if err != nil {
			continue
		}
		log.Println(NewPath(top))
		for _, l := range links {
			p := &Page{
				Title:  l.Title,
				Parent: top,
			}
			if p.Title == t {
				return NewPath(p), nil
			}
			if _, ok := visited[p.Title]; !ok {
				queue = append(queue, p)
				visited[p.Title] = true
			}
		}
	}
	return nil, errors.New("no path between")
}

func main() {
	if len(os.Args) < 3 {
		log.Fatal("start and end article must be specified")
	}
	path, err := Walk(os.Args[1], os.Args[2])
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(path)
}
