package crawlbase

import (
	"bytes"
	"net/http"
	"net/url"
	"testing"
)
import "github.com/PuerkitoBio/goquery"

var links []string = []string{
	"http://google.com",
	"http://test.google.com/3",
	"http://mail.google.com/3",
	"http://www.google.de/3",
	"http://test.google.de/3",
}

func TestGetHrefs(t *testing.T) {
	str := "<a href='1'></a><a href='1'></a>"
	ioreader := bytes.NewReader([]byte(str))
	doc, _ := goquery.NewDocumentFromReader(ioreader)
	testUrl, _ := url.Parse("http://test.com")
	links := GetHrefs(doc, testUrl, false)
	t.Log(links)
	if len(links) != 1 {
		t.Error("duplicate url")
	}
}

func TestGetAllHrefs(t *testing.T) {
	str := "<a href='1'></a><a href='2' style='display:none;'></a>"
	ioreader := bytes.NewReader([]byte(str))
	doc, _ := goquery.NewDocumentFromReader(ioreader)
	testUrl, _ := url.Parse("http://test.com")
	links := GetHrefs(doc, testUrl, true)
	if len(links) != 2 {
		t.Error("incorrect link count")
	}
}

func TestGetVisibleHrefs(t *testing.T) {
	str := "<a href='1'></a><a href='2' style='display:none;'></a>"
	ioreader := bytes.NewReader([]byte(str))
	doc, _ := goquery.NewDocumentFromReader(ioreader)
	testUrl, _ := url.Parse("http://test.com")
	links := GetHrefs(doc, testUrl, false)
	if len(links) == 1 {
		t.Error("incorrect link count")
	}
}

func TestLocationFromPage(t *testing.T) {
	p := &Page{}
	p.Response = &PageResponse{}
	p.Response.StatusCode = 301
	p.Response.Header = http.Header{}
	p.Response.Header.Add("Location", "/test/test3")

	pUrl, _ := url.Parse("http://google.com/q/qe/t?m=5")

	_, locUrl := LocationFromPage(p, pUrl)
	if locUrl != "http://google.com/test/test3" {
		t.Error("location path invalid")
	}
}

func TestGetUrlsFromText(t *testing.T) {
	text := `
	nonulrs:
	/sdf77
	
	urls:
	//test.ifempty.net/jquery.cookie.js
	http://test.ifempty.com/widget
	https://ifempty/kk
	https://ifempty82/kk
	https://test.com
	//ifempty82/kk
	//ifempty/m
	`
	urls := GetUrlsFromText(text, 10)
	//t.Log(len(urls))
	//t.Log(urls)
	if len(urls) != 7 {
		t.Error("too many/less urls found in text")
	}
}

func BenchmarkGetUrlsFromText(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GetUrlsFromText("sjfdkkkkkkkfjs//www.google.com sdfdfjskjjkljfd", -1)
	}
}

func TestGetDomain(t *testing.T) {
	domain := GetDomain("www.ifempty.de")
	if domain != "ifempty.de" {
		t.Error("error in GetDomain")
	}

	domain = GetDomain("ifempty.de")
	if domain != "ifempty.de" {
		t.Error("error in GetDomain")
	}

	domain = GetDomain("localhost")
	if domain != "localhost" {
		t.Error("error in GetDomain")
	}
}

func TestCrawlerAddLinks(t *testing.T) {
	cw := NewCrawler()

	bUrl, _ := url.Parse("http://google.com/35325")

	cw.AddLinks(links, bUrl)
	if len(cw.Links) != 3 {
		t.Error("incorrect link count")
	}
}

func TestCrawlerRemoveLinksNotSameHost(t *testing.T) {
	cw := NewCrawler()
	bUrl, _ := url.Parse("http://google.com/35325")

	// add all links + remove links not same host
	// is the same as addLinks with base url
	cw.AddAllLinks(links)
	cw.RemoveLinksNotSameHost(bUrl)

	if len(cw.Links) != 3 {
		t.Error("incorrect link count")
	}
}
