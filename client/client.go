package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	blog "go_grpc_blog/api"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type Stats struct {
	mu           sync.Mutex
	SuccessCount int64
	ErrorCount   int64
	TimeoutCount int64
	Times        []time.Duration
	AvgTime      time.Duration
}

func (s *Stats) Add(duration time.Duration, err error) {
	if err != nil {
		if err == context.DeadlineExceeded {
			atomic.AddInt64(&s.TimeoutCount, 1)
		}
		atomic.AddInt64(&s.ErrorCount, 1)
		return
	}

	atomic.AddInt64(&s.SuccessCount, 1)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Times = append(s.Times, duration)
}

func calculateMean(times []time.Duration) time.Duration {
	if len(times) == 0 {
		return 0
	}

	var sum time.Duration
	for _, t := range times {
		sum += t
	}
	return sum / time.Duration(len(times))
}

func (s Stats) String() string {
	total := s.SuccessCount + s.ErrorCount
	meanTime := calculateMean(s.Times)
	successRate := float64(s.SuccessCount) / float64(total) * 100

	return fmt.Sprintf(
		"Success: %d, Errors: %d, Success rate: %f \n"+
			"Avg response time: %d",
		s.SuccessCount, s.ErrorCount, successRate, meanTime,
	)
}

func createPost(client *http.Client, wg *sync.WaitGroup, stats *Stats) {
	defer wg.Done()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	reqBody := blog.CreatePostRequest{
		Body: fmt.Sprintf("Post body %d", rand.Intn(1000)),
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		stats.Add(0, err)
		return
	}

	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, "POST", "http://localhost:8080/v1/posts", bytes.NewBuffer(jsonBody))
	if err != nil {
		stats.Add(0, err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Grpc-metadata-user-id", "user-1")

	resp, err := client.Do(req)
	duration := time.Since(start)

	if err != nil {
		stats.Add(duration, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		stats.Add(duration, fmt.Errorf("unexpected status: %d", resp.StatusCode))
		return
	}

	stats.Add(duration, nil)
}

func getPosts(client *http.Client, wg *sync.WaitGroup, stats *Stats) {
	defer wg.Done()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	limit := rand.Intn(10) + 1
	offset := rand.Intn(5)

	reqBody := blog.GetPostsRequest{
		Limit:  int32(limit),
		Offset: int32(offset),
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		stats.Add(0, err)
		return
	}

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, "POST", "http://localhost:8080/v1/posts", bytes.NewBuffer(jsonBody))
	if err != nil {
		stats.Add(0, err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Grpc-metadata-user-id", "user-1")

	resp, err := client.Do(req)
	duration := time.Since(start)

	if err != nil {
		stats.Add(duration, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		stats.Add(duration, fmt.Errorf("unexpected status: %d", resp.StatusCode))
		return
	}

	var posts []blog.Post
	if err := json.NewDecoder(resp.Body).Decode(&posts); err != nil {
		stats.Add(duration, err)
		return
	}

	stats.Add(duration, nil)
}

func main() {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	var createStats Stats
	var wg sync.WaitGroup

	log.Println("Starting creation phase...")
	start := time.Now()

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go createPost(client, &wg, &createStats)
	}

	wg.Wait()
	log.Printf("Creation phase completed in %v\n", time.Since(start))
	log.Println(createStats.String())

	// Phase 2: Get posts
	var getStats Stats

	log.Println("Starting retrieval phase...")
	start = time.Now()

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go getPosts(client, &wg, &getStats)
	}

	wg.Wait()
	log.Printf("Retrieval phase completed in %v\n", time.Since(start))
	log.Println(getStats.String())
}
