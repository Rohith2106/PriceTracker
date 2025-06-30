// backend/tracker/tracker.go
package tracker

import (
	"encoding/json"
	"fmt"
	"log"
	"price-tracker-backend/scraper"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
)

type Tracker struct {
	ID             string
	URL            string
	Selector       string // The selector that initially worked
	ThresholdPrice float64
	Subscription   webpush.Subscription
	StopChan       chan struct{}
	LastPrice      float64
}

func (t *Tracker) StartMonitoring(interval time.Duration) {
	log.Printf("Starting monitoring for ID %s, URL: %s, Threshold: %.2f", t.ID, t.URL, t.ThresholdPrice)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Printf("Checking price for ID %s, URL: %s", t.ID, t.URL)
			// Scrape using the initially successful selector first
			currentPrice, err := scraper.ScrapePriceWithSelector(t.URL, t.Selector)
			if err != nil {
				log.Printf("Error scraping (with specific selector) for %s: %v. Trying general scrape.", t.URL, err)
				// Fallback to general scrape if the specific selector fails (e.g., site structure changed)
				var newSelector string
				currentPrice, newSelector, err = scraper.ScrapePrice(t.URL)
				if err != nil {
					log.Printf("Error during fallback general scrape for %s: %v", t.URL, err)
					continue // Skip this check
				}
				if newSelector != t.Selector && newSelector != "" {
					log.Printf("Selector for %s changed from '%s' to '%s'", t.URL, t.Selector, newSelector)
					t.Selector = newSelector // Update the selector if a new one worked
				}
			}

			log.Printf("Current price for %s: %.2f (Last: %.2f, Threshold: %.2f)", t.URL, currentPrice, t.LastPrice, t.ThresholdPrice)

			if currentPrice > 0 && currentPrice < t.LastPrice && currentPrice <= t.ThresholdPrice {
				log.Printf("PRICE DROP ALERT for %s! New Price: %.2f (Threshold: %.2f)", t.URL, currentPrice, t.ThresholdPrice)
				t.sendNotification(fmt.Sprintf("Price Drop! Now %.2f", currentPrice), fmt.Sprintf("Item at %s is now %.2f!", TruncateURL(t.URL, 40), currentPrice))
				t.LastPrice = currentPrice // Update last price to avoid repeated alerts for same drop
				// Optionally, stop tracking after one alert or make it configurable
				// close(t.StopChan)
				// return
			} else if currentPrice > 0 {
				// Update last price even if no alert, for next comparison
				if currentPrice != t.LastPrice {
					log.Printf("Price for %s updated from %.2f to %.2f", t.URL, t.LastPrice, currentPrice)
					t.LastPrice = currentPrice
				}
			}

		case <-t.StopChan:
			log.Printf("Stopping monitoring for ID %s, URL: %s", t.ID, t.URL)
			return
		}
	}
}

func (t *Tracker) sendNotification(title, body string) {
	// Payload for the push notification
	// Can be a simple string or a JSON object for more structured data
	payload, err := json.Marshal(map[string]interface{}{
		"title": title,
		"body":  body,
		"icon":  "/vite.svg", // Path relative to service worker scope
		"url":   t.URL,       // URL to open on notification click
	})
	if err != nil {
		log.Printf("Error marshalling push payload: %v", err)
		return
	}

	// Send Notification (TTL in seconds, 0 means default)
	resp, err := webpush.SendNotification(payload, &t.Subscription, &webpush.Options{
		TTL: 60 * 60, // Time To Live: 1 hour
		// VAPIDPublicKey:  main.vapidPublicKey, // Already set globally
		// VAPIDPrivateKey: main.vapidPrivateKey,
		// Urgency: webpush.UrgencyHigh, // Optional
	})
	if err != nil {
		log.Printf("Error sending push notification for %s: %v", t.URL, err)
		if resp != nil {
			log.Printf("Push server response: Status %d, Body: %s", resp.StatusCode, resp.Body)
			// If subscription is invalid (e.g., 404, 410), we should stop tracking for this subscription
			if resp.StatusCode == 404 || resp.StatusCode == 410 {
				log.Printf("Subscription for %s seems invalid. Stopping tracker.", t.URL)
				close(t.StopChan) // This will stop the goroutine
				// TODO: Need a way to remove it from the main activeTrackers map
			}
		}
		return
	}
	defer resp.Body.Close()
	log.Printf("Push notification sent successfully for %s! Status: %d", t.URL, resp.StatusCode)
}

// Helper to make URLs shorter for notifications
func TruncateURL(urlStr string, maxLen int) string {
	if len(urlStr) <= maxLen {
		return urlStr
	}
	// Basic truncation, could be smarter (e.g. keep domain)
	return urlStr[:maxLen-3] + "..."
}
