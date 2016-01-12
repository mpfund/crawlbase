package crawlbase

import(
	"github.com/PuerkitoBio/goquery"
	"net/url"
	"crypto/sha256"
	"encoding/hex"
)

type FormInput struct {
	Name  string
	Value string
}

type Form struct {
	Url    string
	Method string
	Inputs []FormInput
}

type Link struct {
	Url  string
	Type string
}

type Page struct {
	Url          string
	CrawlTime    int
	Hrefs        []string
	Forms        []Form
	Links        []Link
	RespCode     int
	RespDuration int
	CrawlerId    int
	Uid          string
	Body         string
}


func GetLinks(doc *goquery.Document,baseUrl *url.URL)[]Link{
	links := []Link{}
	doc.Find("link").Each(func(i int, s *goquery.Selection) {
		link := Link{}
		href, exists := s.Attr("href")
		if exists{
			link.Url = href;
		}
		linkType, exists := s.Attr("type")
		if exists{
			link.Type = linkType;
		}
		links = append(links,link)
	})
	return links
}

func GetHrefs(doc *goquery.Document,baseUrl *url.URL)[]string{
	hrefs := []string{}
	hrefsTest := map[string]bool{}
	
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists {
			var fullUrl = ToAbsUrl(baseUrl, href)
			_, isAlreadyAdded := hrefsTest[fullUrl]
			if !isAlreadyAdded {
				hrefsTest[fullUrl] = true
				hrefs = append(hrefs, fullUrl)
			}
		}
	})
	return hrefs
}

func GetFormUrls(doc *goquery.Document,baseUrl *url.URL)[]Form{
	forms := []Form{}
	
	doc.Find("form").Each(func(i int, s *goquery.Selection) {
		form := Form{}
		href, exists := s.Attr("action")
		if exists{
			form.Url = href;
		}
		method, exists := s.Attr("method")
		if exists{
			form.Method = method
		}
		form.Inputs = []FormInput{}
		s.Find("input").Each(func(i int, s *goquery.Selection){
			input := FormInput{}
			name, exists := s.Attr("name")
			if exists{
				input.Name = name
			}
			value, exists := s.Attr("value")
			if exists{
				input.Value = value
			}
			form.Inputs = append(form.Inputs,input)
		})
		
		forms = append(forms,form)
	})
	return forms
}


func ToAbsUrl(baseurl *url.URL, weburl string) string {
	relurl, err := url.Parse(weburl)
	if err != nil {
		return ""
	}
	absurl := baseurl.ResolveReference(relurl)
	return absurl.String()
}

func ToSha256(message string) string {
	h := sha256.New()
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}