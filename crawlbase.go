package crawlbase

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/BlackEspresso/htmlcheck"
	"github.com/PuerkitoBio/goquery"
	"github.com/miekg/dns"
)

type Page struct {
	URL          string
	CrawlTime    int
	RespDuration int // in milliseconds
	CrawlerId    int
	Uid          string
	Response     *PageResponse
	Request      *PageRequest
	RespInfo     ResponseInfo
	Error        string
	ResponseBody []byte `json:"-"`
	RequestBody  []byte `json:"-"`
}

type PageResponse struct {
	Header        http.Header
	Proto         string
	StatusCode    int
	ContentLength int64
	ContentMIME   string
	Cookies       []Cookie
}

type PageRequest struct {
	Header        http.Header
	Proto         string
	ContentLength int64
	Cookies       []Cookie
}

type ResponseInfo struct {
	Hrefs      []string
	Forms      []Form
	Ressources []Ressource
	JSInfo     []JSInfo
	Requests   []Ressource
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
	Header              http.Header
	Client              http.Client
	IncludeHiddenLinks  bool
	WaitBetweenRequests int
	Links               map[string]bool
	BeforeCrawlFn       func(string) (string, error)
	AfterCrawlFn        func(*Page, error) ([]string, error)
	ValidSchemes        []string
	PageCount           uint64
}

type DNSScanner struct {
	config *dns.ClientConfig
}

var headerUserAgentChrome string = "Mozilla/5.0 (Windows NT 6.3; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/47.0.2526.106 Safari/537.36"

var ErrorCheckRedirect = errors.New("dont redirect")

func NewCrawler() *Crawler {
	cw := Crawler{}
	cw.Client = http.Client{}
	cw.Client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return ErrorCheckRedirect
	}
	cw.Header = http.Header{}
	cw.Header.Set("User-Agent", headerUserAgentChrome)
	cw.Client.Timeout = 30 * time.Second
	cw.WaitBetweenRequests = 1 * 1000
	cw.Links = map[string]bool{}
	cw.ValidSchemes = []string{"http", "https"}
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

func (c *Crawler) GetPage(crawlUrl, method string) (*Page, error) {
	timeStart := time.Now()
	req, err := http.NewRequest(method, crawlUrl, nil)
	if err != nil {
		log.Println("GetPage ", err)
		return nil, err
	}

	for k, v := range c.Header {
		req.Header.Set(k, v[0])
	}

	res, err := c.Client.Do(req)

	timeDur := time.Now().Sub(timeStart)
	page := c.PageFromResponse(req, res, timeDur)

	if err != nil {
		urlerror, ok := err.(*url.Error)
		if !ok || urlerror.Err != ErrorCheckRedirect {
			log.Println("GetPageAfterRequest ", err, res)
			page.Error = err.Error()
			return page, err
		}
	}

	return page, nil
}

func (cw *Crawler) FetchSites(startUrl *url.URL) error {
	cw.AddAllLinks([]string{startUrl.String()})

	crawlStartUrlFirst := false
	if !cw.IsCrawled(startUrl.String()) {
		crawlStartUrlFirst = true
	} else {
		log.Println("start url is already cralwed, skipping: ", startUrl.String())
	}

	for {
		urlStr := ""
		found := false
		if !crawlStartUrlFirst {
			urlStr, found = cw.GetNextLink()
		} else {
			urlStr = startUrl.String()
			crawlStartUrlFirst = false
			found = true
		}

		if !found {
			log.Println("no more links. crawled ", cw.PageCount, "page(s).")
			return nil // done
		}

		if cw.BeforeCrawlFn != nil {
			url, err := cw.BeforeCrawlFn(urlStr)
			if err != nil {
				return err
			}
			urlStr = url
		}

		cw.Links[urlStr] = true

		nextUrl, err := url.Parse(urlStr)
		if err != nil {
			log.Println("error while parsing url: " + err.Error())
			continue
		}
		if !cw.IsValidScheme(nextUrl) {
			log.Println("scheme invalid, skipping url:" + nextUrl.String())
			continue
		}

		log.Println("fetching site: " + urlStr)

		ht, err := cw.GetPage(urlStr, "GET")

		userLinks := []string{}
		if cw.AfterCrawlFn != nil {
			userLinks, err = cw.AfterCrawlFn(ht, err)
			if err != nil {
				log.Println("error, AfterCrawlFn", err)
				return err
			}
		}

		cw.SavePage(ht)
		cw.PageCount += 1

		cw.AddLinks(ht.RespInfo.Hrefs, startUrl)
		cw.AddLinks(userLinks, startUrl)

		time.Sleep(time.Duration(cw.WaitBetweenRequests) * time.Millisecond)
	}
}

func (cw *Crawler) IsCrawled(url string) bool {
	val, hasLink := cw.Links[url]
	if hasLink && val == true {
		return true
	}
	return false
}

func (cw *Crawler) AddCrawledLinks(links []string) {
	for _, newLink := range links {
		cw.Links[newLink] = true
	}
}

func (cw *Crawler) AddAllLinks(links []string) {
	for _, newLink := range links {
		isCrawled := cw.IsCrawled(newLink)
		cw.Links[newLink] = isCrawled
	}
}

func (cw *Crawler) AddLinks(links []string, startUrl *url.URL) {
	for _, newLink := range links {

		newLinkUrl, err := url.Parse(newLink)
		if err != nil {
			continue
		}
		if IsSameDomain(startUrl, newLinkUrl) {
			cw.AddAllLinks([]string{newLink})
		}
	}
}

func (cw *Crawler) IsValidScheme(url *url.URL) bool {
	return ContainsString(cw.ValidSchemes, url.Scheme)
}

func (cw *Crawler) PageFromData(data []byte, url *url.URL) *Page {
	page := Page{}

	page.ResponseBody = data

	ioreader := bytes.NewReader(data)
	doc, err := goquery.NewDocumentFromReader(ioreader)
	if err != nil {
		log.Println("PageFromData: ", err)
	}

	if err == nil {
		hrefs := GetHrefs(doc, url, !cw.IncludeHiddenLinks)
		page.RespInfo.Hrefs = hrefs
		page.RespInfo.Forms = GetFormUrls(doc, url)
		page.RespInfo.Ressources = GetRessources(doc, url)
	}

	page.Response = &PageResponse{}
	page.Request = &PageRequest{}

	return &page
}

func (c *Crawler) PageFromResponse(req *http.Request, res *http.Response, timeDur time.Duration) *Page {
	page := &Page{}
	page.Response = &PageResponse{}
	page.Request = &PageRequest{}

	body := []byte{}

	var err error = nil

	if res != nil {
		body, err = ioutil.ReadAll(res.Body)
		if err == nil {
			page = c.PageFromData(body, req.URL)
		}

		page.Response.ContentMIME = GetContentMime(res.Header)
		page.Response.StatusCode = res.StatusCode
		page.Response.Header = res.Header
		page.Response.Proto = res.Proto

		isRedirect, location := LocationFromPage(page, req.URL)
		if isRedirect {
			hasLocation := ContainsString(page.RespInfo.Hrefs, location)
			if !hasLocation {
				page.RespInfo.Hrefs = append(page.RespInfo.Hrefs, location)
			}
		}
	}

	page.CrawlTime = int(time.Now().Unix())
	page.URL = req.URL.String()
	page.Uid = ToHash(page.URL)
	page.RespDuration = int(timeDur.Seconds() * 1000)
	page.Request.Header = req.Header
	page.Request.Proto = req.Proto
	page.Request.ContentLength = req.ContentLength

	return page
}

func GetContentMime(header http.Header) string {
	contentMIME := strings.Split(header.Get("Content-Type"), ";")[0]
	if contentMIME == "" {
		contentMIME = "text/html"
	}
	return contentMIME
}

func ContainsString(arr []string, key string) bool {
	for _, x := range arr {
		if x == key {
			return true
		}
	}
	return false
}

func (c *Crawler) GetNextLink() (string, bool) {
	for i, l := range c.Links {
		if l == false {
			return i, true
		}
	}
	return "", false
}

func (cw *Crawler) LoadPages(folderpath string) (int, error) {
	files, err := GetPageInfoFiles(folderpath)
	if err != nil {
		log.Fatal(err)
	}

	readCount := 0

	for _, file := range files {
		p, err := LoadPage(file, false)
		if err != nil {
			return readCount, err
		}

		cw.AddCrawledLinks([]string{p.URL})
		cw.AddAllLinks(p.RespInfo.Hrefs)
		readCount += 1
	}
	return readCount, nil
}

func (cw *Crawler) RemoveLinksNotSameHost(baseUrl *url.URL) {
	for k, _ := range cw.Links {
		pUrl, err := url.Parse(k)
		if err != nil || !IsSameDomain(baseUrl, pUrl) {
			delete(cw.Links, k)
		}
	}
}

func IsSameDomain(baseUrl *url.URL, testUrl *url.URL) bool {
	return GetDomain(baseUrl.Host) == GetDomain(testUrl.Host)
}

func GetDomain(host string) string {
	splitted := strings.Split(host, ".")
	lenSplitted := len(splitted)
	if lenSplitted >= 2 {
		return splitted[lenSplitted-2] + "." + splitted[lenSplitted-1]
	}
	if lenSplitted >= 1 {
		return splitted[0]
	}
	return host
}

func LocationFromPage(page *Page, baseUrl *url.URL) (bool, string) {
	if page.Response.StatusCode >= 300 && page.Response.StatusCode < 308 {
		loc := page.Response.Header.Get("Location")
		loc = ToAbsUrl(baseUrl, loc)
		return true, loc
	}
	return false, ""
}

func GetPageInfoFiles(folder string) ([]string, error) {
	files, err := ioutil.ReadDir(folder)
	paths := []string{}
	if err != nil {
		return paths, err
	}

	for _, file := range files {
		isHttpi := strings.HasSuffix(file.Name(), ".httpi")
		if !isHttpi {
			continue
		}
		paths = append(paths, path.Join(folder, file.Name()))
	}
	return paths, nil
}

func LoadPage(filepath string, withContent bool) (*Page, error) {
	content, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	page := Page{}
	err = json.Unmarshal(content, &page)
	if err != nil {
		return nil, err
	}
	if withContent {
		respbinfile := strings.Replace(filepath, ".httpi", ".respbin", 1)
		respbinContent, err := ioutil.ReadFile(respbinfile)
		if err != nil {
			log.Println(err)
		}
		page.ResponseBody = respbinContent
	}

	return &page, nil
}

func (c *Crawler) SavePage(page *Page) {
	if page == nil {
		log.Fatal("SavePage: page is null")
	}
	_, err := os.Stat("./storage")
	if err != nil && os.IsNotExist(err) {
		err := os.Mkdir("storage", 0777)
		checkError(err)
	}

	fileName := strconv.FormatInt(int64(page.CrawlTime), 10)
	err = ioutil.WriteFile("./storage/"+fileName+".respbin", page.ResponseBody, 0666)
	checkError(err)

	content, err := json.MarshalIndent(page, "", "  ")
	checkError(err)
	err = ioutil.WriteFile("./storage/"+fileName+".httpi", content, 0666)

	/*content, err = json.MarshalIndent(page.RespInfo, "", "  ")
	checkError(err)
	err = ioutil.WriteFile("./storage/"+fileName+".httpInfo", content, 0666)
	*/
}

func checkError(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

var regFindUrl *regexp.Regexp = regexp.MustCompile("//[a-zA-Z0-9.-]+/?[a-zA-Z0-9+&@#/%?=~_|!:,.;]*")

func GetUrlsFromText(text []byte, max int) [][]byte {
	return regFindUrl.FindAll(text, max)
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

func (ds *DNSScanner) LoadConfigFromFile(name string) error {
	var err error
	ds.config, err = dns.ClientConfigFromFile(name)
	return err
}

func (ds *DNSScanner) ScanDNS(subdomains []string, name string) map[string][]string {
	dnsResult := map[string][]string{}

	for _, k := range subdomains {
		name := strings.TrimSpace(k + "." + name)
		dnsResult[k], _ = ds.ResolveDNS(name)
	}

	return dnsResult
}

func (ds *DNSScanner) ResolveDNS(name string) ([]string, error) {
	c := new(dns.Client)

	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(name), dns.TypeANY)
	m.RecursionDesired = true

	r, _, err := c.Exchange(m, net.JoinHostPort(ds.config.Servers[0], ds.config.Port))
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

func ToHash(message string) string {
	h := sha1.New()
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}
