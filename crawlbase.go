package crawlbase

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/BlackEspresso/htmlcheck"
	"github.com/PuerkitoBio/goquery"
	"github.com/miekg/dns"
)

type Page struct {
	URL          string
	CrawlTime    int
	RespCode     int
	RespDuration int
	CrawlerId    int
	Uid          string
	RespInfo     ResponseInfo
}

type ResponseInfo struct {
	Body       string
	Hrefs      []string
	Forms      []Form
	Ressources []Ressource
	JSInfo     []JSInfo
	Cookies    []Cookie
	Requests   []Ressource
	TextUrls   []string
	HtmlErrors []*htmlcheck.ValidationError
}

type FormInput struct {
	Name  string
	Type  string
	Value string
}

type Form struct {
	Url    string
	Method string
	Inputs []FormInput
}

type Cookie struct {
	Name     string
	Value    string
	Domain   string
	Httponly bool
}

type Ressource struct {
	Url  string
	Type string
	Rel  string
	Tag  string
}

type JSInfo struct {
	Source string
	Value  string
}

type Crawler struct {
	Header             http.Header
	Client             http.Client
	Validator          htmlcheck.Validator
	IncludeHiddenLinks bool
}

var headerUserAgentChrome string = "Mozilla/5.0 (Windows NT 6.3; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/47.0.2526.106 Safari/537.36"

func NewCrawler() *Crawler {
	cw := Crawler{}
	cw.Client = http.Client{}
	cw.Header = http.Header{}
	cw.Header.Set("User-Agent", headerUserAgentChrome)
	cw.Client.Timeout = 30 * time.Second
	cw.Validator = htmlcheck.Validator{}
	return &cw
}

func LoadTagsFromFile(path string) ([]*htmlcheck.ValidTag, error) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var validTags []*htmlcheck.ValidTag
	err = json.Unmarshal(content, &validTags)

	if err != nil {
		return nil, err
	}

	return validTags, nil
}

func WriteTagsToFile(tags []*htmlcheck.ValidTag, path string) error {
	b, err := json.Marshal(tags)
	if err != nil {
		return err
	}
	ioutil.WriteFile(path, b, 755)
	return nil
}

func (c *Crawler) GetPage(url, method string) (*Page, error) {
	timeStart := time.Now()
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	for k, v := range c.Header {
		req.Header.Set(k, v[0])
	}

	res, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	timeDur := time.Now().Sub(timeStart)

	page := c.PageFromResponse(req, res, timeDur)
	return page, nil
}

func (c *Crawler) PageFromData(data []byte, url *url.URL) *Page {
	page := Page{}

	page.RespInfo.Body = string(data)
	ioreader := bytes.NewReader(data)
	doc, err := goquery.NewDocumentFromReader(ioreader)
	page.RespInfo.TextUrls = GetUrlsFromText(page.RespInfo.Body)
	page.RespInfo.HtmlErrors = c.Validator.ValidateHtmlString(page.RespInfo.Body)

	if err == nil {
		hrefs := GetHrefs(doc, url, !c.IncludeHiddenLinks)
		page.RespInfo.Hrefs = hrefs
		page.RespInfo.Forms = GetFormUrls(doc, url)
		page.RespInfo.Ressources = GetRessources(doc, url)
	}
	return &page
}

func (c *Crawler) PageFromResponse(req *http.Request, res *http.Response, timeDur time.Duration) *Page {
	body, err := ioutil.ReadAll(res.Body)
	page := &Page{}
	if err == nil {
		page = c.PageFromData(body, req.URL)
	}

	page.CrawlTime = int(time.Now().Unix())
	page.URL = req.URL.String()
	page.Uid = ToSha256(page.URL)
	page.RespCode = res.StatusCode
	page.RespDuration = int(timeDur.Seconds() * 1000)
	return page
}

func GetUrlsFromText(text string) []string {
	r, err := regexp.Compile("((https?|ftp|file):)?//[-a-zA-Z0-9+&@#/%?=~_|!:,.;]*[a-zA-Z0-9+&@#/%=~_|]")
	if err != nil {
		log.Fatal(err)
	}
	return r.FindAllString(text, -1)
}

func GetRessources(doc *goquery.Document, baseUrl *url.URL) []Ressource {
	ressources := []Ressource{}
	doc.Find("link").Each(func(i int, s *goquery.Selection) {
		link := Ressource{}
		link.Tag = "link"
		if href, exists := s.Attr("href"); exists {
			link.Url = ToAbsUrl(baseUrl, href)
		}
		if linkType, exists := s.Attr("type"); exists {
			link.Type = linkType
		}
		if rel, exists := s.Attr("rel"); exists {
			link.Rel = rel
		}
		ressources = append(ressources, link)
	})
	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		img := Ressource{}
		img.Tag = "img"
		if href, exists := s.Attr("src"); exists {
			img.Url = ToAbsUrl(baseUrl, href)
		}
		ressources = append(ressources, img)
	})

	doc.Find("script").Each(func(i int, s *goquery.Selection) {
		script := Ressource{}
		script.Tag = "script"
		if href, exists := s.Attr("src"); exists {
			script.Url = ToAbsUrl(baseUrl, href)
		} else {
			return
		}
		if scriptType, exists := s.Attr("type"); exists {
			script.Type = scriptType
		}
		ressources = append(ressources, script)
	})
	doc.Find("style").Each(func(i int, s *goquery.Selection) {
		style := Ressource{}
		style.Tag = "style"
		if href, exists := s.Attr("src"); exists {
			style.Url = ToAbsUrl(baseUrl, href)
		} else {
			return
		}
		if styleType, exists := s.Attr("type"); exists {
			style.Type = styleType
		}
		ressources = append(ressources, style)
	})
	return ressources
}

func GetStylesCss(style string) map[string]string {
	splitted := strings.Split(style, ";")
	attrs := map[string]string{}
	for _, k := range splitted {
		kv := strings.Split(k, ":")
		if len(kv) > 1 {
			key := strings.TrimSpace(kv[0])
			value := strings.TrimSpace(kv[1])
			attrs[key] = value
		}
	}
	return attrs
}

func IsVisibleCss(style string) bool {
	styles := GetStylesCss(style)
	display, hasDisplay := styles["display"]
	visibilty, hasVisibilty := styles["visibility"]
	if hasDisplay && display == "none" {
		return false
	}
	if hasVisibilty && visibilty == "hidden" {
		return false
	}
	return true
}

func GetHrefs(doc *goquery.Document, baseUrl *url.URL, removeInvisibles bool) []string {
	hrefs := []string{}
	hrefsTest := map[string]bool{}

	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists {
			style, hasStyle := s.Attr("style")
			if removeInvisibles && hasStyle {
				isVisible := IsVisibleCss(style)
				if isVisible {
					return
				}
			}

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

func GetFormUrls(doc *goquery.Document, baseUrl *url.URL) []Form {
	forms := []Form{}

	doc.Find("form").Each(func(i int, s *goquery.Selection) {
		form := Form{}
		if href, exists := s.Attr("action"); exists {
			form.Url = ToAbsUrl(baseUrl, href)
		}
		if method, exists := s.Attr("method"); exists {
			form.Method = method
		}
		form.Inputs = []FormInput{}
		s.Find("input").Each(func(i int, s *goquery.Selection) {
			input := FormInput{}
			if name, exists := s.Attr("name"); exists {
				input.Name = name
			}
			if value, exists := s.Attr("value"); exists {
				input.Value = value
			}
			if inputType, exists := s.Attr("type"); exists {
				input.Type = inputType
			}

			form.Inputs = append(form.Inputs, input)
		})

		forms = append(forms, form)
	})
	return forms
}

func resolveDNS(name string) ([]string, error) {
	config, _ := dns.ClientConfigFromFile("./resolv.conf")
	c := new(dns.Client)

	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(name), dns.TypeANY)
	m.RecursionDesired = true

	r, _, err := c.Exchange(m, net.JoinHostPort(config.Servers[0], config.Port))
	if err != nil {
		return nil, err
	}

	resp := []string{}

	for _, v := range r.Answer {
		resp = append(resp, v.String())
	}

	return resp, nil
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
