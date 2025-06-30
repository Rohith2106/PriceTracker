// backend/scraper/scraper.go
package scraper

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// PriceSelectorConfig holds selectors for different domains or general patterns
type PriceSelectorConfig struct {
	Domain   string // e.g., "amazon.com"
	Selector string // goquery selector string
	// Potentially add more fields like attribute to get, or if it's split into multiple elements
}

// Define some common selectors. This list needs to be expanded and refined.
var commonSelectors = []string{
	".a-price.a-text-price .a-offscreen", // More specific Amazon selector
	".a-price-whole",                     // Amazon integer part (needs fraction too)
	".a-offscreen",                       // Amazon screen-reader price
	"[itemprop='price']",
	".price",
	".product-price",
	".current-price",
	"#priceblock_ourprice",     // Older Amazon
	"#priceblock_dealprice",    // Older Amazon deal
	".priceInfo .price .value", // Example more specific
	// Add more based on target sites
}

// ScrapePrice tries to find and parse a price from a given URL.
// It returns the price, the selector that worked, and any error.
func ScrapePrice(urlStr string) (float64, string, error) {
	log.Printf("Scraping URL: %s", urlStr)
	res, err := http.Get(urlStr)
	if err != nil {
		return 0, "", fmt.Errorf("failed to get URL: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return 0, "", fmt.Errorf("bad status: %s", res.Status)
	}

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return 0, "", fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Try Amazon specific logic first for .a-price-whole
	amazonPriceText := ""
	doc.Find(".a-price-whole").EachWithBreak(func(i int, s *goquery.Selection) bool {
		wholePart := strings.TrimSpace(s.Text())
		wholePart = strings.ReplaceAll(wholePart, ",", "") // 1,234 -> 1234
		wholePart = strings.TrimSuffix(wholePart, ".")     // 1234. -> 1234

		fractionPart := "00"
		if fractionEl := s.SiblingsFiltered(".a-price-fraction"); fractionEl.Length() > 0 {
			fractionPart = strings.TrimSpace(fractionEl.First().Text())
		}
		amazonPriceText = wholePart + "." + fractionPart
		return false // Stop after first match
	})

	if amazonPriceText != "" {
		price, err := ParsePriceString(amazonPriceText)
		if err == nil {
			log.Printf("Found Amazon price: %f using .a-price-whole", price)
			return price, ".a-price-whole (composite)", nil
		}
	}

	// Try general selectors
	for _, selector := range commonSelectors {
		priceText := ""
		doc.Find(selector).EachWithBreak(func(i int, s *goquery.Selection) bool {
			// Prioritize elements with text content. Some might be meta tags.
			text := strings.TrimSpace(s.Text())
			if text != "" {
				priceText = text
				return false // Found, break
			}
			// Check for content attribute if text is empty (e.g., <meta itemprop="price" content="29.99">)
			if contentVal, exists := s.Attr("content"); exists && strings.TrimSpace(contentVal) != "" {
				priceText = strings.TrimSpace(contentVal)
				return false // Found, break
			}
			return true // Continue
		})

		if priceText != "" {
			price, err := ParsePriceString(priceText)
			if err == nil {
				log.Printf("Found price: %f using selector: %s", price, selector)
				return price, selector, nil
			}
			log.Printf("Failed to parse '%s' from selector '%s': %v", priceText, selector, err)
		}
	}

	return 0, "", fmt.Errorf("could not find or parse price on page with known selectors")
}

// ScrapePriceWithSelector scrapes a price from a URL using a specific selector.
func ScrapePriceWithSelector(urlStr, selector string) (float64, error) {
	res, err := http.Get(urlStr)
	if err != nil {
		return 0, fmt.Errorf("failed to get URL: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return 0, fmt.Errorf("bad status: %s", res.Status)
	}

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Special handling for Amazon composite selector
	if selector == ".a-price-whole (composite)" {
		// ... (logic for Amazon price)
		amazonPriceText := ""
		doc.Find(".a-price-whole").EachWithBreak(func(i int, s *goquery.Selection) bool {
			wholePart := strings.TrimSpace(s.Text())
			wholePart = strings.ReplaceAll(wholePart, ",", "")
			wholePart = strings.TrimSuffix(wholePart, ".")

			fractionPart := "00"
			if fractionEl := s.SiblingsFiltered(".a-price-fraction"); fractionEl.Length() > 0 {
				fractionPart = strings.TrimSpace(fractionEl.First().Text())
			}
			amazonPriceText = wholePart + "." + fractionPart
			return false
		})
		if amazonPriceText != "" {
			return ParsePriceString(amazonPriceText)
		}
	}

	priceText := ""
	doc.Find(selector).EachWithBreak(func(i int, s *goquery.Selection) bool {
		text := strings.TrimSpace(s.Text())
		if text != "" {
			priceText = text
			return false
		}
		if contentVal, exists := s.Attr("content"); exists && strings.TrimSpace(contentVal) != "" {
			priceText = strings.TrimSpace(contentVal)
			return false
		}
		return true
	})

	if priceText == "" {
		return 0, fmt.Errorf("could not find price with selector: %s", selector)
	}

	return ParsePriceString(priceText)
}

func ParsePriceString(priceStr string) (float64, error) {
	// Remove currency symbols, thousands separators, etc.
	// Be careful with different decimal separators if supporting international sites.
	// This is a simplified parser.
	replacer := strings.NewReplacer("$", "", "€", "", "£", "", "₹", "", ",", "", " ", "")
	cleanedStr := replacer.Replace(priceStr)

	// If it was "1.234,56" (European), convert to "1234.56"
	// This is a naive check, proper localization is complex.
	if strings.Count(cleanedStr, ".") > 1 && strings.Contains(cleanedStr, ",") {
		cleanedStr = strings.ReplaceAll(cleanedStr, ".", "")
		cleanedStr = strings.ReplaceAll(cleanedStr, ",", ".")
	} else if strings.Contains(cleanedStr, ",") && !strings.Contains(cleanedStr, ".") {
		// Handle cases like "1,23" -> "1.23"
		if strings.LastIndex(cleanedStr, ",") == len(cleanedStr)-3 {
			cleanedStr = strings.ReplaceAll(cleanedStr, ",", ".")
		}
	}

	price, err := strconv.ParseFloat(cleanedStr, 64)
	if err != nil {
		return 0, fmt.Errorf("could not parse '%s' (cleaned: '%s') as float: %w", priceStr, cleanedStr, err)
	}
	return price, nil
}
