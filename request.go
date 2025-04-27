package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/google/go-querystring/query"
	jsoniter "github.com/json-iterator/go"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode"
)

type RequestParam struct {
	Q            string `url:"q" json:"q"`
	GoogleDomain string `url:"google_domain" json:"google_domain"`
	Gl           string `url:"gl,omitempty" json:"gl"`
	Hl           string `url:"hl,omitempty" json:"hl"`
	DirectLink   string `url:"direct_link,omitempty" json:"direct_link"`
	Engine       string `url:"engine,omitempty" json:"engine"`
}

type Response struct {
	Filters         []Filters        `json:"filters,omitempty"`
	ShoppingResults []ShoppingResult `json:"shopping_results,omitempty"`
}

type Filters struct {
	Type      string    `json:"type,omitempty"`
	InputType string    `json:"input_type,omitempty"`
	Options   []Options `json:"option,omitempty"`
}

func (f *Filters) IsEmpty() bool {
	return f.Type == "" && f.InputType == "" && len(f.Options) == 0
}

type Options struct {
	Text string `json:"text,omitempty"`
	Tbs  string `json:"tbs,omitempty"`
	Link string `json:"link,omitempty"`
}

func (o *Options) IsEmpty() bool {
	return o.Text == "" && o.Tbs == "" && o.Link == ""
}

type InlineShoppingResult struct {
	Position       int      `json:"position,omitempty"`
	BlockPosition  string   `json:"block_position,omitempty"`
	Title          string   `json:"title,omitempty"`
	Price          string   `json:"price,omitempty"`
	ExtractedPrice float64  `json:"extracted_price,omitempty"`
	Link           string   `json:"link,omitempty"`
	Source         string   `json:"source,omitempty"`
	Thumbnail      string   `json:"thumbnail,omitempty"`
	Extensions     []string `json:"extensions,omitempty"`
}

func (i *InlineShoppingResult) IsEmpty() bool {
	return i.Title == "" && i.Price == "" && i.ExtractedPrice == 0 && i.Link == "" && i.Source == "" && i.Thumbnail == "" && len(i.Extensions) == 0
}

type ShoppingResult struct {
	Position            int      `json:"position,omitempty"`
	Title               string   `json:"title,omitempty"`
	Link                string   `json:"link,omitempty"`
	ProductLink         string   `json:"product_link,omitempty"`
	ProductID           string   `json:"product_id,omitempty"`
	Source              string   `json:"source,omitempty"`
	SourceIcon          string   `json:"source_icon,omitempty"`
	Price               string   `json:"price,omitempty"`
	ExtractedPrice      float64  `json:"extracted_price,omitempty"`
	OldPrice            string   `json:"old_price,omitempty"`
	ExtractedOldPrice   float64  `json:"extracted_old_price,omitempty"`
	SecondHandCondition string   `json:"second_hand_condition,omitempty"`
	Rating              float64  `json:"rating,omitempty"`
	Reviews             int      `json:"reviews,omitempty"`
	Snippet             string   `json:"snippet,omitempty"`
	Extensions          []string `json:"extensions,omitempty"`
	Badge               string   `json:"badge,omitempty"`
	Thumbnail           string   `json:"thumbnail,omitempty"`
	Thumbnails          []string `json:"thumbnails,omitempty"`
	Tag                 string   `json:"tag,omitempty"`
	Delivery            string   `json:"delivery,omitempty"`
	StoreRating         float64  `json:"store_rating,omitempty"`
	StoreReviews        int      `json:"store_reviews,omitempty"`
}

func (s ShoppingResult) IsEmpty() bool {
	return s.Position == 0 &&
		s.Title == "" &&
		s.Link == "" &&
		s.ProductLink == "" &&
		s.ProductID == "" &&
		s.Source == "" &&
		s.SourceIcon == "" &&
		s.Price == "" &&
		s.ExtractedPrice == 0 &&
		s.OldPrice == "" &&
		s.ExtractedOldPrice == 0 &&
		s.Rating == 0 &&
		s.Reviews == 0 &&
		len(s.Extensions) == 0 &&
		s.Badge == "" &&
		s.Thumbnail == "" &&
		len(s.Thumbnails) == 0 &&
		s.Tag == "" &&
		s.Delivery == "" &&
		s.StoreRating == 0 &&
		s.StoreReviews == 0
}

func DoShopping(params RequestParam, proxy string) Response {
	initProxyClient(proxy)
	urlQuery, _ := query.Values(params)
	urlQuery.Del("google_domain")
	urlQuery.Del("engine")
	urlQuery.Set("udm", "28")
	urlQuery.Set("sclient", "sclient=gws-wiz-modeless-shopping")
	urlStr := fmt.Sprintf("https://www.%s/search?", params.GoogleDomain) + urlQuery.Encode()
	req, reqError := http.NewRequest("GET", urlStr, nil)
	if reqError != nil {
		log.Fatal(reqError)
	}
	setHeader(req)
	resp, respError := c.Do(req)
	if respError != nil {
		panic(fmt.Sprintf("Failed to get data through proxy, proxy:%s, err:%s", proxy, respError.Error()))
	}
	if resp.StatusCode != http.StatusOK {
		panic(fmt.Sprintf("request %s fail :httpcode(%v)", "google_shopping", resp.StatusCode))
	}
	defer resp.Body.Close()
	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	response := parseData(bodyText, params)
	return response
}

func parseData(bodyText []byte, params RequestParam) Response {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(bodyText))
	if err != nil {
		panic(fmt.Sprintf("Read body failed, err:%s", err.Error()))
	}
	var response Response
	// sidebar
	var filterList []Filters
	doc.Find("div[id='appbar'] div[jsname='HJCfLb'] div[jsname='pYVSud'] ul").ChildrenFiltered("li").Each(func(i int, s *goquery.Selection) {
		filter := Filters{
			Type: s.Find("div[jsname='ARU61'] span[role='heading']").Text(), // eg: Refine results
		}
		var optionList []Options
		s.Find("ul[jsname='CbM3zb'] li").Each(func(i int, s1 *goquery.Selection) {
			option := Options{
				Text: s1.Find("a").Text(), // 侧边栏具体的子类 eg: In store
				Link: s1.Find("a").AttrOr("href", ""),
			}
			if option.Link != "" {
				option.Link = fmt.Sprintf("https://www.google.com/%s", option.Link)
			}
			if !option.IsEmpty() {
				optionList = append(optionList, option)
			}
		})
		if len(optionList) > 0 {
			filter.Options = optionList
		}
		if !filter.IsEmpty() {
			filterList = append(filterList, filter)
		}
	})
	if len(filterList) > 0 {
		response.Filters = filterList
	}

	// Create a map to store product links
	productLinks := make(map[string]string)
	// Use WaitGroup to wait for all goroutines to complete
	var wg sync.WaitGroup
	getShoppingResult := func(count int, s3 *goquery.Selection) ShoppingResult {
		s4 := s3.Find("div[jsname='luUKCc'] div[class='MUWJ8c']").Children()
		title := s4.Children().Eq(1).Text() // eg: Folgers Coffee Ground Classic Roast
		if title == "" {
			return ShoppingResult{}
		}
		var catalogId, gpCid, headlineOfferDoCid, imageDoCid, mid string
		detail := s3.Find("div[jsname='dQK82e']")
		catalogId = detail.AttrOr("data-cid", "")
		gpCid = detail.AttrOr("data-gid", "")
		headlineOfferDoCid = detail.AttrOr("data-oid", "")
		imageDoCid = detail.AttrOr("data-iid", "")
		mid = detail.AttrOr("data-mid", "")
		// Define a function to obtain product links concurrently
		fetchProductLink := func(catalogId, gpCid, headlineOfferDoCid, imageDoCid, mid string) {
			defer wg.Done()
			link, err := getShoppingDetail(catalogId, gpCid, headlineOfferDoCid, imageDoCid, mid)
			if err == nil {
				productLinks[catalogId] = link
			}
		}

		// Start a goroutine to get the product link
		wg.Add(1)
		go fetchProductLink(catalogId, gpCid, headlineOfferDoCid, imageDoCid, mid)

		// create shoppingResult
		shoppingResult := ShoppingResult{
			Position:   count,
			Title:      title,
			ProductID:  catalogId,
			Source:     s4.Children().Find("span[class='WJMUdc rw5ecc']").Text(),
			SourceIcon: s4.Children().Eq(3).Find("img").AttrOr("src", ""),
			Price:      s4.Children().Eq(2).Find("span").First().Text(),
			OldPrice:   s4.Children().Eq(2).Find("span").Eq(1).Text(),
			Thumbnail:  s4.Children().Eq(0).Find("img").AttrOr("src", ""),
			Tag:        s4.Children().Eq(0).Text(),
			Delivery:   s4.Children().Find("span[class='ybnj7e']").Text(),
		}

		// Processing price
		if shoppingResult.ProductID != "" {
			shoppingResult.ProductLink = fmt.Sprintf("https://www.google.com/shopping/product/%s?gl=%s", shoppingResult.ProductID, params.Gl)
		}
		if shoppingResult.Price != "" {
			shoppingResult.ExtractedPrice, _ = strconv.ParseFloat(strings.ReplaceAll(strings.ReplaceAll(shoppingResult.Price, "$", ""), ",", ""), 64)
		}
		if shoppingResult.OldPrice != "" {
			shoppingResult.ExtractedOldPrice, _ = strconv.ParseFloat(strings.ReplaceAll(strings.ReplaceAll(shoppingResult.OldPrice, "$", ""), ",", ""), 64)
			if shoppingResult.ExtractedOldPrice == 0.0 {
				shoppingResult.SecondHandCondition = shoppingResult.OldPrice
				shoppingResult.OldPrice = ""
			}
		}

		// Process ratings and comments
		shoppingResult.Rating, _ = strconv.ParseFloat(s4.Children().Find("div[class='LFROUd']").Find("span").First().Children().Eq(0).Text(), 64)
		reviewsText := s4.Children().Find("div[class='LFROUd']").Find("span").First().Children().Eq(2).Text()
		reviewsStr := strings.ReplaceAll(strings.ReplaceAll(reviewsText, "(", ""), ")", "")
		if strings.Contains(reviewsStr, "K") {
			reviews, _ := strconv.ParseFloat(strings.ReplaceAll(reviewsStr, "K", ""), 64)
			shoppingResult.Reviews = int(reviews * 1000)
		} else {
			shoppingResult.Reviews, _ = strconv.Atoi(strings.ReplaceAll(reviewsStr, "K", ""))
		}

		// Add extended information
		extensions := append(shoppingResult.Extensions, s4.Children().Eq(0).Text())
		if len(extensions) > 0 {
			shoppingResult.Extensions = extensions
		}

		// Working with clips and thumbnails
		id := s3.Find("div[jsname='uVFeEd']").AttrOr("id", "")
		html := ButtonsData(id, string(bodyText))
		doc2, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
		shoppingResult.Snippet = doc2.Find("div[jsname='luUKCc']").Eq(1).Text()
		matches := regexp.MustCompile(`google\.ldi\s*=\s*\{([\s\S]*?)};`).FindStringSubmatch(string(bodyText))
		var matchesStr string
		if len(matches) > 1 {
			matchesStr = "{" + matches[1] + "}"
		}
		var imageUrlMap map[string]string
		_ = json.Unmarshal([]byte(matchesStr), &imageUrlMap)
		if imageUrlMap != nil {
			var thumbnails []string
			doc2.Find("div[jsname='DzNtMd']").Each(func(i int, s5 *goquery.Selection) {
				id2 := s5.Find("img").AttrOr("id", "")
				thumbnails = append(thumbnails, imageUrlMap[id2])
			})
			if len(thumbnails) > 0 {
				shoppingResult.Thumbnails = thumbnails
			}
		}

		return shoppingResult
	}

	// Below more places
	var count int
	var shoppingResultList []ShoppingResult
	doc.Find("div[jscontroller='wuEeed']").Each(func(i int, s2 *goquery.Selection) {

		s2.Find("g-card[jscontroller='XT8Clf'] ul li").Each(func(i int, s3 *goquery.Selection) {
			count++
			shoppingResult := getShoppingResult(count, s3)
			if !shoppingResult.IsEmpty() {
				shoppingResultList = append(shoppingResultList, shoppingResult)
			}
		})
		// Wait for all goroutines to finish
		wg.Wait()
		// Write back the link value
		for i := range shoppingResultList {
			link := productLinks[shoppingResultList[i].ProductID]
			shoppingResultList[i].Link = link
		}
	})
	if len(shoppingResultList) > 0 {
		response.ShoppingResults = shoppingResultList
	}

	var count2 int
	var inLineResultList []InlineShoppingResult
	doc.Find("div[id='rso'] g-scrolling-carousel[jscontroller='pgCXqb']").Find("div[jsname='s2gQvd'] div[jsname='U8yK8']").Each(func(i int, s5 *goquery.Selection) {
		title := s5.Find("div[class='orXoSd'] div[role='heading']").Find("a").Text()
		if title == "" {
			return
		}
		count2++
		inLineResult := InlineShoppingResult{
			Position:      count2,
			BlockPosition: "top",
			Title:         title,
			Price:         s5.Find("div[class='orXoSd']").Find("div[class='T4OwTb']").Text(),
			Link:          s5.Find("a").First().AttrOr("href", ""),
			Source:        "",
			Thumbnail:     s5.Find("img").First().AttrOr("src", ""),
		}
		if inLineResult.Price != "" {
			inLineResult.ExtractedPrice, _ = strconv.ParseFloat(extractNumbersUsingMap(inLineResult.Price), 64)
		}
		if inLineResult.Link != "" {
			inLineResult.Link = fmt.Sprintf("https://www.google.com/%s", inLineResult.Link)
		}
		var extensions []string
		extensions = append(extensions, s5.Find("div[class='orXoSd'] div[class='LbUacb']").Text())
		extensions = append(extensions, s5.Find("div[class='orXoSd']").Children().Eq(1).Text())
		if len(inLineResult.Extensions) > 0 {
			inLineResult.Extensions = extensions
		}
		if !inLineResult.IsEmpty() {
			inLineResultList = append(inLineResultList, inLineResult)
		}
	})
	return response
}

func ButtonsData(id string, html string) string {
	const regular = "window\\.jsl\\.dh\\('%s',(.*?)\\);"
	ariaControlsZZ := fmt.Sprintf(regular, id)
	regex, _ := regexp.Compile(ariaControlsZZ)
	matches := regex.FindAllStringSubmatch(html, -1)
	var jsonString string
	if len(matches) > 0 {
		for _, value := range matches {
			jsonString = value[1]
		}
	}
	newJson := strings.ReplaceAll(jsonString, "'", "")
	decodedString := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(newJson, `\x3c`, "<"), `\x3d`, "="), `\x22`, `"`), `\x3e`, ">")
	return decodedString
}

func extractNumbersUsingMap(s string) string {
	filter := func(r rune) rune {
		if unicode.IsDigit(r) || r == '.' {
			return r
		}
		return -1
	}
	return strings.Map(filter, s)
}

func getShoppingDetail(catalogId, gpCid, headlineOfferDocId, imageDocId, mid string) (productLink string, err error) {
	async := fmt.Sprintf("catalogId:%s,gpCid:%s,headlineOfferDocId:%s,imageDocId:%s,mid:%s,_fmt:jspb", catalogId, gpCid, headlineOfferDocId, imageDocId, mid)
	urlStr := fmt.Sprintf("https://www.google.com/async/oapv?async=%s", async)
	req, reqError := http.NewRequest("GET", urlStr, nil)
	if reqError != nil {
		return "", reqError
	}
	resp, respError := c.Do(req)
	if respError != nil {
		return "", reqError
	}
	if resp.StatusCode != http.StatusOK {
		return "", reqError
	}
	defer resp.Body.Close()
	bodyText, err := io.ReadAll(resp.Body)
	// Convert []byte to string
	str := string(bodyText)

	// Split string by line
	lines := strings.Split(str, "\n")

	// Check whether the second line exists
	if len(lines) >= 2 {
		// Get the second line
		secondLine := lines[1]
		// Parsing JSON data using jsoniter
		offer := jsoniter.Get([]byte(secondLine), "ProductDetailsResult").ToString()
		var link string
		link = jsoniter.Get([]byte(offer), 81, 0, 19, 1, 3).ToString()
		if link == "" {
			link = jsoniter.Get([]byte(offer), 81, 0, 0, 0, 26, 0, 29).ToString()
		}
		if link == "" {
			link = jsoniter.Get([]byte(offer), 81, 0, 0, 2, 2, 0).ToString()
		}
		return link, nil
	} else {
		return "", fmt.Errorf("second line does not exist")
	}
}

func setHeader(req *http.Request) {
	req.Header.Set("accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("accept-language", "en")
	req.Header.Set("cache-control", "no-cache")
	req.Header.Set("downlink", "10")
	req.Header.Set("pragma", "no-cache")
	req.Header.Set("priority", "u=0, i")
	req.Header.Set("referer", "https://www.google.com/")
	req.Header.Set("rtt", "200")
	req.Header.Set("sec-ch-prefers-color-scheme", "light")
	req.Header.Set("sec-ch-ua", `"Not(A:Brand";v="99", "Google Chrome";v="133", "Chromium";v="133"`)
	req.Header.Set("sec-ch-ua-arch", `"x86"`)
	req.Header.Set("sec-ch-ua-bitness", `"64"`)
	req.Header.Set("sec-ch-ua-form-factors", `"Desktop"`)
	req.Header.Set("sec-ch-ua-full-version", `"133.0.6943.98"`)
	req.Header.Set("sec-ch-ua-full-version-list", `"Not(A:Brand";v="99.0.0.0", "Google Chrome";v="133.0.6943.98", "Chromium";v="133.0.6943.98"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-model", `""`)
	req.Header.Set("sec-ch-ua-platform", `"Windows"`)
	req.Header.Set("sec-ch-ua-platform-version", `"10.0.0"`)
	req.Header.Set("sec-ch-ua-wow64", "?0")
	req.Header.Set("sec-fetch-dest", "document")
	req.Header.Set("sec-fetch-mode", "navigate")
	req.Header.Set("sec-fetch-site", "same-origin")
	req.Header.Set("sec-fetch-user", "?1")
	req.Header.Set("upgrade-insecure-requests", "1")
	req.Header.Set("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36")
	req.Header.Set("x-browser-channel", "stable")
	req.Header.Set("x-browser-copyright", "Copyright 2025 Google LLC. All rights reserved.")
	req.Header.Set("x-browser-validation", "1nAW9Rb/M8Lkk97ILDg00FWYjns=")
	req.Header.Set("x-browser-year", "2025")
}

var c *http.Client

func initProxyClient(proxy string) {
	// proxy addr
	proxyUrl, err := url.Parse(proxy)
	if err != nil {
		panic(err)
	}

	// custom Transport
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyUrl),
	}

	// create Client
	client := &http.Client{
		Transport: transport,
	}
	c = client
}
