package pkg

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gookit/color"
	"github.com/headzoo/surf"
	"github.com/headzoo/surf/browser"
	"github.com/labstack/echo/v4"
)

type CrawlOptions struct {
	TimeoutMillisecond int64
	SleepMillisecond   int64
	URL                string
	IgnoreQueries      bool
	Depth              int
	LimitUrls          int
	LimitEmails        int
	WriteToFile        string
	CrawlFromFile      bool
	MaxWorkers         int
}

type CrawlOption func(*CrawlOptions) error

type HTTPChallenge struct {
	browse *browser.Browser

	urls             []string
	Emails           []string
	TotalURLsCrawled int
	TotalURLsFound   int
	options          *CrawlOptions
}

func NewHTTPChallenge(opts ...CrawlOption) *HTTPChallenge {
	opt := &CrawlOptions{}
	for _, o := range opts {
		err := o(opt)
		if err != nil {
			panic(err)
		}
	}
	b := surf.NewBrowser()
	b.SetUserAgent("GO kevincobain2000/email_extractor")
	b.SetTimeout(time.Duration(opt.TimeoutMillisecond) * time.Millisecond)

	return &HTTPChallenge{
		browse:  b,
		options: opt,
	}
}

func (hc *HTTPChallenge) CrawlRecursiveParallel(url string, wg *sync.WaitGroup) *HTTPChallenge {
	defer wg.Done()
	urls := hc.Crawl(url)

	var mu sync.Mutex
	for _, u := range urls {
		// Only apply limits if not crawling from file
		if !hc.options.CrawlFromFile {
			if len(hc.urls) >= hc.options.LimitUrls {
				break
			}
			if len(hc.Emails) >= hc.options.LimitEmails {
				hc.Emails = hc.Emails[:hc.options.LimitEmails]
				break
			}
		}
		if StringInSlice(u, hc.urls) {
			continue
		}

		mu.Lock()
		hc.urls = append(hc.urls, u)
		mu.Unlock()

		if runtime.NumGoroutine() > 10000 {
			color.Warn.Print("Sleeping")
			color.Secondary.Print("....................")
			color.Secondary.Println(fmt.Sprintf("%ds (goroutines %d, exceeded limit)", 10, runtime.NumGoroutine()))
			time.Sleep(10 * time.Second)
			wg.Add(1)
			go hc.CrawlRecursiveParallel(u, wg)
		} else {
			wg.Add(1)
			go hc.CrawlRecursiveParallel(u, wg)
		}
	}
	return hc
}
func (hc *HTTPChallenge) CrawlRecursive(url string) *HTTPChallenge {
	urls := hc.Crawl(url)

	for _, u := range urls {
		// Only apply limits if not crawling from file
		if !hc.options.CrawlFromFile {
			if len(hc.urls) >= hc.options.LimitUrls {
				break
			}
			if len(hc.Emails) >= hc.options.LimitEmails {
				hc.Emails = hc.Emails[:hc.options.LimitEmails]
				break
			}
		}
		if StringInSlice(u, hc.urls) {
			continue
		}

		hc.urls = append(hc.urls, u)

		hc.CrawlRecursive(u)
	}
	return hc
}

func (hc *HTTPChallenge) CrawlRecursiveStream(url string, c echo.Context, enc *json.Encoder) *HTTPChallenge {
	urls := hc.Crawl(url)

	for _, u := range urls {
		select {
		case <-c.Request().Context().Done():
			color.Secondary.Print("API.........................")
			color.Warn.Println("Request Cancelled")
			return nil
		default:
		}

		// Only apply limits if not crawling from file
		if !hc.options.CrawlFromFile {
			if len(hc.urls) >= hc.options.LimitUrls {
				c.Request().Context().Done()
				return hc
			}
			if len(hc.Emails) >= hc.options.LimitEmails {
				hc.Emails = hc.Emails[:hc.options.LimitEmails]
				c.Request().Context().Done()
				return hc
			}
		}
		if StringInSlice(u, hc.urls) {
			continue
		}
		if IsAnAsset(u) {
			continue
		}
		p := "status" + "_SPLIT_DELIMETER_" + u
		err := enc.Encode(p)
		if err != nil {
			color.Secondary.Print("API.........................")
			color.Danger.Println(err.Error())
		}
		c.Response().Flush()

		hc.urls = append(hc.urls, u)

		err = hc.browse.Head(url)
		if err != nil {
			continue
		}
		if !strings.HasPrefix(hc.browse.ResponseHeaders().Get("Content-Type"), "text/html") {
			continue
		}
		err = hc.browse.Open(u)
		if err != nil {
			color.Secondary.Print("API.........................")
			color.Danger.Println(err.Error())
			continue
		}

		rawBody := hc.browse.Body()

		emails := ExtractEmailsFromText(rawBody)
		emails = FilterOutCommonExtensions(emails)
		emails = UniqueStrings(emails)
		hc.Emails = append(hc.Emails, emails...)
		for _, email := range emails {
			p := email + "_SPLIT_DELIMETER_" + u
			err := enc.Encode(p)
			if err != nil {
				color.Secondary.Print("API.........................")
				color.Danger.Println(err.Error())
			}
			c.Response().Flush()
		}

		hc.CrawlRecursiveStream(u, c, enc)
	}
	return hc
}

func (hc *HTTPChallenge) Crawl(url string) []string {
	// check if url doesn't end with pdf, png or jpg
	if IsAnAsset(url) {
		return []string{}
	}

	if hc.options.SleepMillisecond > 0 {
		color.Secondary.Print("Sleeping")
		color.Secondary.Print("....................")
		color.Secondary.Println(fmt.Sprintf("%dms (sleeping before request)", hc.options.SleepMillisecond))
		time.Sleep(time.Duration(hc.options.SleepMillisecond) * time.Millisecond)
	}
	urls := []string{}
	err := hc.browse.Head(url)
	if err != nil {
		return urls
	}
	if !strings.HasPrefix(hc.browse.ResponseHeaders().Get("Content-Type"), "text/html") {
		return urls
	}

	err = hc.browse.Open(url)
	if err != nil {
		return urls
	}

	hc.TotalURLsCrawled++

	color.Secondary.Print("Crawling")
	color.Secondary.Print("....................")
	if hc.browse.StatusCode() >= 400 {
		color.Danger.Print(hc.browse.StatusCode())
	} else {
		color.Success.Print(hc.browse.StatusCode())
	}
	color.Secondary.Println(" " + url)
	rawBody := hc.browse.Body()

	emails := ExtractEmailsFromText(rawBody)
	emails = FilterOutCommonExtensions(emails)
	emails = UniqueStrings(emails)
	if len(emails) > 0 {
		hc.TotalURLsFound++
		color.Note.Print("Emails")
		color.Secondary.Print("......................")
		color.Note.Println(fmt.Sprintf("(%d) %s", len(emails), url))
		for _, email := range emails {
			color.Note.Print("Emails")
			color.Secondary.Print("......................")
			color.Success.Println(email)
		}
		fmt.Println()
	}
	if hc.options.WriteToFile != "" {
		hc.Emails = append(hc.Emails, emails...)
		hc.Emails = UniqueStrings(hc.Emails)
	}

	// crawl the page and print all links
	hc.browse.Find("a").Each(func(_ int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}
		href = RelativeToAbsoluteURL(href, url, GetBaseURL(url))

		if hc.options.IgnoreQueries {
			href = RemoveAnyQueryParam(href)
		}
		href = RemoveAnyAnchors(href)
		isSubset := IsSameDomain(hc.options.URL, href)
		if !isSubset {
			return
		}

		if hc.options.Depth != -1 {
			depth := URLDepth(href, hc.options.URL)
			if depth == -1 {
				return
			}
			if depth == 0 {
				return
			}
			if depth > hc.options.Depth {
				return
			}
		}
		urls = append(urls, href)
	})
	urls = UniqueStrings(urls)
	return urls
}

func (hc *HTTPChallenge) CrawlSingleURL(url string) *HTTPChallenge {
	// check if url doesn't end with pdf, png or jpg
	if IsAnAsset(url) {
		return hc
	}

	if hc.options.SleepMillisecond > 0 {
		color.Secondary.Print("Sleeping")
		color.Secondary.Print("....................")
		color.Secondary.Println(fmt.Sprintf("%dms (sleeping before request)", hc.options.SleepMillisecond))
		time.Sleep(time.Duration(hc.options.SleepMillisecond) * time.Millisecond)
	}

	err := hc.browse.Head(url)
	if err != nil {
		return hc
	}
	if !strings.HasPrefix(hc.browse.ResponseHeaders().Get("Content-Type"), "text/html") {
		return hc
	}

	err = hc.browse.Open(url)
	if err != nil {
		return hc
	}

	hc.TotalURLsCrawled++

	color.Secondary.Print("Crawling")
	color.Secondary.Print("....................")
	if hc.browse.StatusCode() >= 400 {
		color.Danger.Print(hc.browse.StatusCode())
	} else {
		color.Success.Print(hc.browse.StatusCode())
	}
	color.Secondary.Println(" " + url)
	rawBody := hc.browse.Body()

	emails := ExtractEmailsFromText(rawBody)
	emails = FilterOutCommonExtensions(emails)
	emails = UniqueStrings(emails)
	if len(emails) > 0 {
		hc.TotalURLsFound++
		color.Note.Print("Emails")
		color.Secondary.Print("......................")
		color.Note.Println(fmt.Sprintf("(%d) %s", len(emails), url))
		for _, email := range emails {
			color.Note.Print("Emails")
			color.Secondary.Print("......................")
			color.Success.Println(email)
		}
		fmt.Println()
	}

	// Add emails to memory
	hc.Emails = append(hc.Emails, emails...)
	hc.Emails = UniqueStrings(hc.Emails)

	// Save emails to file immediately if output file is specified
	if hc.options.WriteToFile != "" && len(emails) > 0 {
		// Append new emails to file
		err := AppendEmailsToFile(emails, hc.options.WriteToFile)
		if err != nil {
			color.Danger.Print("File write")
			color.Secondary.Print("....................")
			color.Danger.Println("Error writing emails to file:", err)
		}
	}

	return hc
}

func (hc *HTTPChallenge) CrawlSingleURLParallel(url string, wg *sync.WaitGroup) *HTTPChallenge {
	defer wg.Done()

	// check if url doesn't end with pdf, png or jpg
	if IsAnAsset(url) {
		return hc
	}

	if hc.options.SleepMillisecond > 0 {
		color.Secondary.Print("Sleeping")
		color.Secondary.Print("....................")
		color.Secondary.Println(fmt.Sprintf("%dms (sleeping before request)", hc.options.SleepMillisecond))
		time.Sleep(time.Duration(hc.options.SleepMillisecond) * time.Millisecond)
	}

	err := hc.browse.Head(url)
	if err != nil {
		return hc
	}
	if !strings.HasPrefix(hc.browse.ResponseHeaders().Get("Content-Type"), "text/html") {
		return hc
	}

	err = hc.browse.Open(url)
	if err != nil {
		return hc
	}

	var mu sync.Mutex
	mu.Lock()
	hc.TotalURLsCrawled++
	mu.Unlock()

	color.Secondary.Print("Crawling")
	color.Secondary.Print("....................")
	if hc.browse.StatusCode() >= 400 {
		color.Danger.Print(hc.browse.StatusCode())
	} else {
		color.Success.Print(hc.browse.StatusCode())
	}
	color.Secondary.Println(" " + url)
	rawBody := hc.browse.Body()

	emails := ExtractEmailsFromText(rawBody)
	emails = FilterOutCommonExtensions(emails)
	emails = UniqueStrings(emails)
	if len(emails) > 0 {
		mu.Lock()
		hc.TotalURLsFound++
		mu.Unlock()
		color.Note.Print("Emails")
		color.Secondary.Print("......................")
		color.Note.Println(fmt.Sprintf("(%d) %s", len(emails), url))
		for _, email := range emails {
			color.Note.Print("Emails")
			color.Secondary.Print("......................")
			color.Success.Println(email)
		}
		fmt.Println()
	}

	mu.Lock()
	hc.Emails = append(hc.Emails, emails...)
	hc.Emails = UniqueStrings(hc.Emails)
	mu.Unlock()

	// Save emails to file immediately if output file is specified
	if hc.options.WriteToFile != "" && len(emails) > 0 {
		mu.Lock()
		err := AppendEmailsToFile(emails, hc.options.WriteToFile)
		mu.Unlock()
		if err != nil {
			color.Danger.Print("File write")
			color.Secondary.Print("....................")
			color.Danger.Println("Error writing emails to file:", err)
		}
	}

	return hc
}

func (hc *HTTPChallenge) CrawlSingleURLWithBrowser(url string, browser *browser.Browser) *HTTPChallenge {
	// check if url doesn't end with pdf, png or jpg
	if IsAnAsset(url) {
		return hc
	}

	if hc.options.SleepMillisecond > 0 {
		color.Secondary.Print("Sleeping")
		color.Secondary.Print("....................")
		color.Secondary.Println(fmt.Sprintf("%dms (sleeping before request)", hc.options.SleepMillisecond))
		time.Sleep(time.Duration(hc.options.SleepMillisecond) * time.Millisecond)
	}

	err := browser.Head(url)
	if err != nil {
		// Force garbage collection after failed request
		runtime.GC()
		return hc
	}
	if !strings.HasPrefix(browser.ResponseHeaders().Get("Content-Type"), "text/html") {
		// Force garbage collection for non-HTML content
		runtime.GC()
		return hc
	}

	err = browser.Open(url)
	if err != nil {
		// Force garbage collection after failed request
		runtime.GC()
		return hc
	}

	var mu sync.Mutex
	mu.Lock()
	hc.TotalURLsCrawled++
	mu.Unlock()

	color.Secondary.Print("Crawling")
	color.Secondary.Print("....................")
	if browser.StatusCode() >= 400 {
		color.Danger.Print(browser.StatusCode())
	} else {
		color.Success.Print(browser.StatusCode())
	}
	color.Secondary.Println(" " + url)

	// Get body content before clearing browser
	rawBody := browser.Body()

	emails := ExtractEmailsFromText(rawBody)
	emails = FilterOutCommonExtensions(emails)
	emails = UniqueStrings(emails)

	if len(emails) > 0 {
		mu.Lock()
		hc.TotalURLsFound++
		mu.Unlock()
		color.Note.Print("Emails")
		color.Secondary.Print("......................")
		color.Note.Println(fmt.Sprintf("(%d) %s", len(emails), url))
		for _, email := range emails {
			color.Note.Print("Emails")
			color.Secondary.Print("......................")
			color.Success.Println(email)
		}
		fmt.Println()
	}

	mu.Lock()
	hc.Emails = append(hc.Emails, emails...)
	hc.Emails = UniqueStrings(hc.Emails)
	mu.Unlock()

	// Save emails to file immediately if output file is specified
	if hc.options.WriteToFile != "" && len(emails) > 0 {
		mu.Lock()
		err := AppendEmailsToFile(emails, hc.options.WriteToFile)
		mu.Unlock()
		if err != nil {
			color.Danger.Print("File write")
			color.Secondary.Print("....................")
			color.Danger.Println("Error writing emails to file:", err)
		}
	}

	// CRITICAL: Force garbage collection to free all memory after processing
	runtime.GC()

	// Additional memory cleanup
	rawBody = ""
	emails = nil

	return hc
}

func (hc *HTTPChallenge) GetURLsCount() int {
	return len(hc.urls)
}

func (hc *HTTPChallenge) GetEmailsCount() int {
	return len(hc.Emails)
}

func (hc *HTTPChallenge) AddURL(url string) {
	hc.urls = append(hc.urls, url)
}

func (hc *HTTPChallenge) HasURL(url string) bool {
	return StringInSlice(url, hc.urls)
}

func (hc *HTTPChallenge) CrawlURLsWithWorkerPool(urls []string) {
	if hc.options.MaxWorkers <= 0 {
		hc.options.MaxWorkers = 10 // Reduced default to 10 workers to save memory
	}

	// Create channels for job distribution
	urlChan := make(chan string, len(urls))
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < hc.options.MaxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			processedCount := 0
			for url := range urlChan {
				if hc.HasURL(url) {
					continue
				}
				hc.AddURL(url)

				// Create NEW browser instance for EACH request to prevent memory accumulation
				b := surf.NewBrowser()
				b.SetUserAgent("GO kevincobain2000/email_extractor")
				b.SetTimeout(time.Duration(hc.options.TimeoutMillisecond) * time.Millisecond)

				hc.CrawlSingleURLWithBrowser(url, b)

				// Browser will be garbage collected after each request
				b = nil

				// Force garbage collection every 50 URLs to free memory more frequently
				processedCount++
				if processedCount%50 == 0 {
					runtime.GC()
				}
			}
		}()
	}

	// Send URLs to workers
	for _, url := range urls {
		urlChan <- url
	}
	close(urlChan)

	// Wait for all workers to complete
	wg.Wait()
}
