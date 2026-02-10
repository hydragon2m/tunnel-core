package stats

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
)

type UsageReport struct {
	UserID   string `json:"user_id"`
	Bytes    int64  `json:"bytes"`
	Requests int    `json:"requests"`
}

type Collector struct {
	adminAPIURL string
	buffer      map[string]*UsageReport
	mu          sync.Mutex
	interval    time.Duration
}

func NewCollector(adminAPIURL string, interval time.Duration) *Collector {
	c := &Collector{
		adminAPIURL: adminAPIURL,
		buffer:      make(map[string]*UsageReport),
		interval:    interval,
	}
	go c.start()
	return c
}

func (c *Collector) Record(userID string, bytes int64, requests int) {
	if userID == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.buffer[userID]; !ok {
		c.buffer[userID] = &UsageReport{UserID: userID}
	}
	c.buffer[userID].Bytes += bytes
	c.buffer[userID].Requests += requests
}

func (c *Collector) start() {
	ticker := time.NewTicker(c.interval)
	for range ticker.C {
		c.flush()
	}
}

func (c *Collector) flush() {
	c.mu.Lock()
	if len(c.buffer) == 0 {
		c.mu.Unlock()
		return
	}
	reports := make([]*UsageReport, 0, len(c.buffer))
	for _, r := range c.buffer {
		reports = append(reports, r)
	}
	c.buffer = make(map[string]*UsageReport)
	c.mu.Unlock()

	for _, r := range reports {
		c.sendReport(r)
	}
}

func (c *Collector) sendReport(r *UsageReport) {
	data, _ := json.Marshal(r)
	resp, err := http.Post(c.adminAPIURL, "application/json", bytes.NewBuffer(data))
	if err != nil {
		log.Printf("❌ Failed to send usage report for %s: %v", r.UserID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("❌ Admin API returned error for usage report: %s", resp.Status)
	}
}
