package crawlbase

import (
	"bytes"
	"net/url"
	"testing"
)
import "github.com/PuerkitoBio/goquery"

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

func TestCrawlerAddLinks(t *testing.T) {
	cw := NewCrawler()
	links := []string{
		"http://google.com",
		"http://google.com/3",
		"http://google.de/3",
	}
	bUrl, _ := url.Parse("http://google.com/35325")

	cw.AddLinks(links, bUrl)
	if len(cw.Links) != 2 {
		t.Error("incorrect link count")
	}
}
