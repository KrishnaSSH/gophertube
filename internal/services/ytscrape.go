package services

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"gophertube/internal/types"
)

// Pre-compiled regex for better performance
var ytInitialDataRegex = regexp.MustCompile(`(?s)var ytInitialData = (\{.*?\});`)

// Optimized HTTP client with better connection pooling
var httpClient = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DisableKeepAlives:   false,
		DisableCompression:  false,
	},
}

// Video extraction with early termination
func extractVideosFromJSON(data []byte, limit int) ([]types.Video, error) {
	m := ytInitialDataRegex.FindSubmatch(data)
	if len(m) < 2 {
		return nil, errors.New("ytInitialData not found")
	}

	var root map[string]interface{}
	if err := json.Unmarshal(m[1], &root); err != nil {
		return nil, err
	}

	videos := make([]types.Video, 0, limit)
	seen := make(map[string]struct{}, limit)

	// Targeted JSON navigation - look for specific paths where videos are stored
	var extractVideos func(interface{}) bool
	extractVideos = func(node interface{}) bool {
		if len(videos) >= limit {
			return true // Early termination
		}

		if m, ok := node.(map[string]interface{}); ok {
			// Check for videoRenderer first (most common)
			if vr, ok := m["videoRenderer"]; ok {
				v := parseVideoRenderer(vr)
				if v.Title != "" && v.URL != "" {
					if _, ok := seen[v.URL]; !ok {
						seen[v.URL] = struct{}{}
						videos = append(videos, v)
						if len(videos) >= limit {
							return true
						}
					}
				}
			}

			// Check for compactVideoRenderer
			if cr, ok := m["compactVideoRenderer"]; ok {
				v := parseVideoRenderer(cr)
				if v.Title != "" && v.URL != "" {
					if _, ok := seen[v.URL]; !ok {
						seen[v.URL] = struct{}{}
						videos = append(videos, v)
						if len(videos) >= limit {
							return true
						}
					}
				}
			}

			// Look for content array which contains videos
			if content, ok := m["contents"]; ok {
				if extractVideos(content) {
					return true
				}
			}

			// Look for tab contents
			if tabs, ok := m["tabs"]; ok {
				if extractVideos(tabs) {
					return true
				}
			}

			// Look for section list renderer
			if sections, ok := m["sectionListRenderer"]; ok {
				if extractVideos(sections) {
					return true
				}
			}

			// Recursively check other map values
			for _, v := range m {
				if extractVideos(v) {
					return true
				}
			}
		} else if arr, ok := node.([]interface{}); ok {
			for _, v := range arr {
				if extractVideos(v) {
					return true
				}
			}
		}
		return false
	}

	extractVideos(root)

	if len(videos) == 0 {
		return nil, errors.New("no videos found")
	}

	return videos, nil
}

func SearchYouTube(query string, limit int, progress func(current, total int)) ([]types.Video, error) {
	if progress != nil {
		progress(0, 2+limit)
	}

	// Single optimized request with best parameters
	url := "https://www.youtube.com/results?search_query=" + urlQueryEscape(query) + "&sp=EgIQAQ%253D%253D&hl=en&gl=US"
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if progress != nil {
		progress(1, 2+limit)
	}

	// Extract videos with early termination
	videos, err := extractVideosFromJSON(body, limit)
	if err != nil {
		return nil, err
	}

	if progress != nil {
		progress(2, 2+limit)
	}

	// If we don't have enough videos, try one more strategy
	if len(videos) < limit {
		altUrl := "https://www.youtube.com/results?search_query=" + urlQueryEscape(query) + "&sp=EgIQAQ%25253D%25253D&hl=en&gl=US"
		altResp, err := httpClient.Get(altUrl)
		if err == nil {
			defer altResp.Body.Close()
			altBody, err := io.ReadAll(altResp.Body)
			if err == nil {
				altVideos, err := extractVideosFromJSON(altBody, limit-len(videos))
				if err == nil {
					// Merge videos avoiding duplicates
					existingURLs := make(map[string]bool)
					for _, v := range videos {
						existingURLs[v.URL] = true
					}

					for _, v := range altVideos {
						if !existingURLs[v.URL] && len(videos) < limit {
							videos = append(videos, v)
							existingURLs[v.URL] = true
						}
					}
				}
			}
		}
	}

	// Ensure we don't exceed limit
	if len(videos) > limit {
		videos = videos[:limit]
	}

	if progress != nil {
		progress(2, 2+limit)
	}

	// Load all thumbnails concurrently for faster loading
	var wg sync.WaitGroup
	var mu sync.Mutex
	done := 0

	for i := 0; i < len(videos); i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			thumbPath := cacheThumbnailOptimized(videos[i].Thumbnail)
			if thumbPath == "" && videos[i].Thumbnail != "" {
				thumbPath = tryFallbackThumbnails(videos[i].Thumbnail)
			}
			videos[i].ThumbnailPath = thumbPath

			mu.Lock()
			done++
			if progress != nil {
				progress(2+done, 2+limit)
			}
			mu.Unlock()
		}(i)
	}

	wg.Wait()
	CleanupHTTPConnections()

	return videos, nil
}

// Optimized fallback thumbnail function
func tryFallbackThumbnails(originalURL string) string {
	fallbackURLs := []string{
		strings.ReplaceAll(originalURL, "default", "hqdefault"),
		strings.ReplaceAll(originalURL, "default", "mqdefault"),
		strings.ReplaceAll(originalURL, "default", "sddefault"),
		strings.ReplaceAll(originalURL, "default", "maxresdefault"),
	}

	for _, fallbackURL := range fallbackURLs {
		if fallbackURL != originalURL {
			thumbPath := cacheThumbnailOptimized(fallbackURL)
			if thumbPath != "" {
				return thumbPath
			}
		}
	}
	return ""
}

func cacheThumbnailOptimized(url string) string {
	if url == "" {
		return ""
	}
	hash := fmt.Sprintf("%x", md5.Sum([]byte(url)))
	cacheDir := "/tmp/gophertube-thumbs"
	os.MkdirAll(cacheDir, 0o755)
	thumbPath := filepath.Join(cacheDir, hash+".jpg")

	if _, err := os.Stat(thumbPath); err == nil {
		return thumbPath
	}

	// Try multiple times with different timeouts
	for attempt := 0; attempt < 3; attempt++ {
		timeout := time.Duration(2+attempt) * time.Second
		client := &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 5,
				IdleConnTimeout:     30 * time.Second,
				DisableCompression:  false,
			},
		}

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			continue
		}

		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
		req.Header.Set("Accept", "image/webp,image/apng,image/*,*/*;q=0.8")
		req.Header.Set("Accept-Encoding", "gzip, deflate")
		req.Header.Set("Connection", "keep-alive")
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("Referer", "https://www.youtube.com/")

		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			continue
		}

		data, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
		if err != nil || len(data) == 0 || isHTML(data) {
			continue
		}

		tempPath := thumbPath + ".tmp"
		if err := ioutil.WriteFile(tempPath, data, 0o644); err != nil {
			continue
		}

		if err := os.Rename(tempPath, thumbPath); err != nil {
			os.Remove(tempPath)
			continue
		}

		return thumbPath
	}

	return ""
}

// CleanupHTTPConnections closes idle connections to prevent memory leaks
func CleanupHTTPConnections() {
	if transport, ok := httpClient.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}
}

func urlQueryEscape(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, " ", "+"), "#", "%23")
}

func parseVideoRenderer(vr interface{}) types.Video {
	m, ok := vr.(map[string]interface{})
	if !ok {
		return types.Video{}
	}
	title := safeJQText(m, "title", "runs", 0, "text")
	videoId := safeJQString(m, "videoId")
	url := "https://www.youtube.com/watch?v=" + videoId
	channel := safeJQText(m, "longBylineText", "runs", 0, "text")
	duration := safeJQString(m, "lengthText", "simpleText")
	views := safeJQString(m, "viewCountText", "simpleText")
	thumb := ""
	if thumbs, ok := m["thumbnail"].(map[string]interface{}); ok {
		if arr, ok := thumbs["thumbnails"].([]interface{}); ok && len(arr) > 0 {
			for i := len(arr) - 1; i >= 0; i-- {
				if t, ok := arr[i].(map[string]interface{}); ok {
					if thumbURL, ok := t["url"].(string); ok && thumbURL != "" {
						if strings.Contains(thumbURL, "maxres") || strings.Contains(thumbURL, "hq") {
							thumb = thumbURL
							break
						} else if thumb == "" {
							thumb = thumbURL
						}
					}
				}
			}
		}
	}

	if thumb == "" {
		if thumbInfo, ok := m["thumbnailInfo"].(map[string]interface{}); ok {
			if thumbURL, ok := thumbInfo["url"].(string); ok && thumbURL != "" {
				thumb = thumbURL
			}
		}
	}

	// Extract published/upload date as relative time
	published := safeJQString(m, "publishedTimeText", "simpleText")

	return types.Video{
		Title:     title,
		URL:       url,
		Author:    channel,
		Duration:  duration,
		Views:     views,
		Thumbnail: thumb,
		Published: published,
	}
}

func safeJQString(m map[string]interface{}, keys ...string) string {
	cur := m
	for i, k := range keys {
		if i == len(keys)-1 {
			if v, ok := cur[k].(string); ok {
				return v
			}
			return ""
		}
		if v, ok := cur[k].(map[string]interface{}); ok {
			cur = v
		} else {
			return ""
		}
	}
	return ""
}

func safeJQText(m map[string]interface{}, k1, k2 string, idx int, k3 string) string {
	if a, ok := m[k1].(map[string]interface{}); ok {
		if arr, ok := a[k2].([]interface{}); ok && len(arr) > idx {
			if t, ok := arr[idx].(map[string]interface{}); ok {
				if s, ok := t[k3].(string); ok {
					return s
				}
			}
		}
	}
	return ""
}

func isHTML(data []byte) bool {
	return bytes.HasPrefix(data, []byte("<!DOCTYPE html")) || bytes.HasPrefix(data, []byte("<html"))
}
