package crawlbase

import (
	"bytes"
	"net/url"
	"testing"
)
import "github.com/PuerkitoBio/goquery"

func TestReverse(t *testing.T) {
	list := []string{"a", "b", "c"}
	list = removeStringInSlice(list, "b")
	t.Log(list)
	if len(list) != 2 {
		t.Error("item not removed")
	}
}

func TestGetHrefs(t *testing.T) {
	str := "<a href='1'></a><a href='1'></a>"
	ioreader := bytes.NewReader([]byte(str))
	doc, _ := goquery.NewDocumentFromReader(ioreader)
	testUrl, _ := url.Parse("http://test.com")
	links := GetHrefs(doc, testUrl)
	t.Log(links)
	if len(links) != 1 {
		t.Error("duplicate url")
	}
}
