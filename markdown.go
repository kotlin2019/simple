package simple

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/iris-contrib/blackfriday"
	"github.com/microcosm-cc/bluemonday"
	"github.com/sirupsen/logrus"
	"github.com/vinta/pangu"
	"regexp"
	"strings"
)

type MdResult struct {
	ContentHtml string // 内容
	SummaryText string // 摘要
	TocHtml     string // TOC目录
	ThumbUrl    string // 缩略图
}

// option
type MdOption func(*SimpleMd)

// 开启toc
func MdWithTOC() MdOption {
	return func(md *SimpleMd) {
		md.toc = true
	}
}

// 开启缩略图
func MdWithThumb(md *SimpleMd) MdOption {
	return func(md *SimpleMd) {
		md.thumb = true
	}
}

// 生成摘要的长度
func MdWithSummaryLength(summaryLength int) MdOption {
	return func(md *SimpleMd) {
		md.summaryTextLength = summaryLength
	}
}

// simple md
type SimpleMd struct {
	summaryTextLength int  // 摘要长度
	toc               bool // 是否开启Toc
	thumb             bool // 是否构建目录
}

// new simple md
func NewMd(options ...MdOption) *SimpleMd {
	simpleMd := &SimpleMd{
		summaryTextLength: 256,
		toc:               false,
		thumb:             false,
	}
	for _, option := range options {
		option(simpleMd)
	}
	return simpleMd
}

// run
func (this *SimpleMd) Run(mdText string) *MdResult {
	mdText = strings.Replace(mdText, "\r\n", "\n", -1)

	var unsafe []byte
	if this.toc {
		htmlRenderer := blackfriday.WithRenderer(blackfriday.NewHTMLRenderer(blackfriday.HTMLRendererParameters{
			Flags: blackfriday.CommonHTMLFlags | blackfriday.TOC,
		}))
		unsafe = blackfriday.Run([]byte([]byte(mdText)), htmlRenderer)
	} else {
		unsafe = blackfriday.Run([]byte(mdText))
	}

	contentHTML := string(unsafe)
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(contentHTML))

	// 处理图片
	// doc.Find("img").Each(func(i int, ele *goquery.Selection) {
	// 	src, _ := ele.Attr("src")
	// 	ele.SetAttr("data-src", src)
	// 	ele.RemoveAttr("src")
	// })

	doc.Find("*").Contents().FilterFunction(func(i int, ele *goquery.Selection) bool {
		if "#text" != goquery.NodeName(ele) {
			return false
		}
		parent := goquery.NodeName(ele.Parent())

		return "span" != parent && "code" != parent && "pre" != parent
	}).Each(func(i int, ele *goquery.Selection) {
		text := ele.Text()
		text = pangu.SpacingText(text)
		ele.ReplaceWithHtml(text)
	})

	doc.Find("code").Each(func(i int, ele *goquery.Selection) {
		code, err := ele.Html()
		if nil != err {
			logrus.Error("get element HTML failed", ele, err)
		} else {
			code = strings.Replace(code, "<", "&lt;", -1)
			code = strings.Replace(code, ">", "&gt;", -1)
			ele.SetHtml(code)
		}
	})

	var tocHtml string
	if this.toc {
		nav := doc.Find("nav").First().Remove()
		tocHtml, _ = nav.Html()
	}

	contentHTML, _ = doc.Find("body").Html()
	contentHTML = bluemonday.UGCPolicy().AllowAttrs("class").Matching(regexp.MustCompile("^language-[a-zA-Z0-9]+$")).OnElements("code").
		AllowAttrs("data-src").OnElements("img").
		AllowAttrs("class", "target", "id", "style").Globally().
		AllowAttrs("src", "width", "height", "border", "marginwidth", "marginheight").OnElements("iframe").
		AllowAttrs("controls", "src").OnElements("audio").
		AllowAttrs("color").OnElements("font").
		AllowAttrs("controls", "src", "width", "height").OnElements("video").
		AllowAttrs("src", "media", "type").OnElements("source").
		AllowAttrs("width", "height", "data", "type").OnElements("object").
		AllowAttrs("name", "value").OnElements("param").
		AllowAttrs("src", "type", "width", "height", "wmode", "allowNetworking").OnElements("embed").
		Sanitize(contentHTML)

	return &MdResult{
		ContentHtml: contentHTML,
		SummaryText: this.summaryText(doc),
		ThumbUrl:    this.thumbnailUrl(doc),
		TocHtml:     tocHtml,
	}
}

// 缩略图
func (this *SimpleMd) thumbnailUrl(doc *goquery.Document) string {
	if !this.thumb {
		return ""
	}
	selection := doc.Find("img").First()
	thumbnailURL, _ := selection.Attr("src")
	if "" == thumbnailURL {
		thumbnailURL, _ = selection.Attr("data-src")
	}
	return thumbnailURL
}

// 摘要
func (this *SimpleMd) summaryText(doc *goquery.Document) string {
	if this.summaryTextLength <= 0 {
		return ""
	}
	text := doc.Text()
	text = strings.TrimSpace(text)
	return GetSummary(text, this.summaryTextLength)
}
