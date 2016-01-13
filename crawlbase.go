package crawlbase

import(
	"github.com/PuerkitoBio/goquery"
	"net/url"
	"crypto/sha256"
	"encoding/hex"
)

type FormInput struct {
	Name  string
	Type string
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
	Rel string
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
		if href, exists := s.Attr("href");exists{
			link.Url = ToAbsUrl(baseUrl,href);
		}
		if linkType, exists := s.Attr("type");exists{
			link.Type = linkType;
		}
		if rel, exists := s.Attr("rel");exists{
			link.Rel = rel;
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
			var fullUrl = ToAbsUrl(baseUrl,href)
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
		if href, exists := s.Attr("action"); exists{
			form.Url = ToAbsUrl(baseUrl,href);
		}
		if method, exists := s.Attr("method");exists{
			form.Method = method
		}
		form.Inputs = []FormInput{}
		s.Find("input").Each(func(i int, s *goquery.Selection){
			input := FormInput{}
			if name, exists := s.Attr("name");exists{
				input.Name = name
			}
			if value, exists := s.Attr("value");exists{
				input.Value = value
			}
			if inputType, exists := s.Attr("type");exists{
				input.Type = inputType
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