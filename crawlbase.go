package crawlbase

type Page struct {
	Url          string
	CrawlTime    int
	Links        []string
	RespCode     int
	RespDuration int
	CrawlerId    int
	Uid          string
	Body         string
}
