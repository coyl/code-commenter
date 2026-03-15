package jobstore

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/rs/zerolog/log"

	"code-commenter/api/internal/ports"
)

// Client reads and writes job data to S3. If bucket is empty, all operations no-op.
type Client struct {
	bucket string
	client *s3.Client
}

// ClientOptions configures S3 client (endpoint and credentials from env).
type ClientOptions struct {
	Bucket    string
	Region    string
	Endpoint  string // e.g. https://minio:9000
	AccessKey string
	SecretKey string
}

// NewClient creates an S3 job store client. If bucket is empty, returns a no-op client.
func NewClient(ctx context.Context, opts ClientOptions) (*Client, error) {
	if opts.Bucket == "" {
		return &Client{}, nil
	}
	region := opts.Region
	if region == "" {
		region = "us-east-1"
	}
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}
	if opts.AccessKey != "" && opts.SecretKey != "" {
		cfg.Credentials = credentials.NewStaticCredentialsProvider(opts.AccessKey, opts.SecretKey, "")
	}
	s3ClientOptions := []func(*s3.Options){}
	if opts.Endpoint != "" {
		endpoint := strings.TrimSuffix(opts.Endpoint, "/")
		// Custom S3-compatible endpoints (e.g. MinIO) usually require path-style
		// addressing: endpoint/bucket/key instead of bucket.endpoint/key.
		s3ClientOptions = append(s3ClientOptions, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true
		})
	}
	return &Client{
		bucket: opts.Bucket,
		client: s3.NewFromConfig(cfg, s3ClientOptions...),
	}, nil
}

// SegmentStored is the shape we store per segment (code HTML, codePlain, narration).
// Audio is stored as separate 0.pcm, 1.pcm, ... files.
type SegmentStored struct {
	Code      string `json:"code"`
	CodePlain string `json:"codePlain"`
	Narration string `json:"narration"`
}

// ResultStored is the JSON we store as result.json (full code + segments + metadata).
type ResultStored struct {
	RawJSON       string          `json:"rawJson"`
	FullCode      string          `json:"fullCode"`
	FullCodePlain string          `json:"fullCodePlain"`
	CSS           string          `json:"css"`
	Title         string          `json:"title"`
	NarrationLang string          `json:"narrationLang"`
	StoryHTML     string          `json:"storyHtml,omitempty"`
	Segments      []SegmentStored `json:"segments"`
	OwnerSub      string          `json:"ownerSub,omitempty"`
	OwnerEmail    string          `json:"ownerEmail,omitempty"`
}

// UploadJob writes prompt, result JSON (with css, title, narrationLang, owner, storyHtml), and per-segment PCM files under jobID/.
func (c *Client) UploadJob(ctx context.Context, jobID, prompt, rawJSON, fullCode, fullCodePlain, css, title, narrationLang, ownerSub, ownerEmail, storyHTML string, segments []SegmentStored, segmentAudio [][]byte) error {
	if c.bucket == "" {
		return nil
	}
	prefix := jobID + "/"

	// prompt.txt
	if err := c.put(ctx, prefix+"prompt.txt", []byte(prompt)); err != nil {
		return err
	}

	// result.json
	result := ResultStored{
		RawJSON:       rawJSON,
		FullCode:      fullCode,
		FullCodePlain: fullCodePlain,
		CSS:           css,
		Title:         title,
		NarrationLang: narrationLang,
		StoryHTML:     storyHTML,
		Segments:      segments,
		OwnerSub:      ownerSub,
		OwnerEmail:    ownerEmail,
	}
	resultBody, err := json.Marshal(result)
	if err != nil {
		return err
	}
	if err := c.put(ctx, prefix+"result.json", resultBody); err != nil {
		return err
	}

	// N.pcm for each segment with audio
	for i, pcm := range segmentAudio {
		if len(pcm) == 0 {
			continue
		}
		key := prefix + strconv.Itoa(i) + ".pcm"
		if err := c.put(ctx, key, pcm); err != nil {
			return err
		}
	}

	log.Info().Str("job", jobID).Int("segments", len(segmentAudio)).Msg("job uploaded to S3")
	return nil
}

// JobLoaded is the response for loading a job (segments with audio as base64).
type JobLoaded struct {
	Prompt        string             `json:"prompt"`
	RawJSON       string             `json:"rawJson"`
	FullCode      string             `json:"fullCode"`
	CSS           string             `json:"css"`
	Title         string             `json:"title"`
	NarrationLang string             `json:"narrationLang"`
	StoryHTML     string             `json:"storyHtml,omitempty"`
	Segments      []SegmentWithAudio `json:"segments"`
}

// SegmentWithAudio has one audio chunk (base64) for playback.
type SegmentWithAudio struct {
	Code        string   `json:"code"`
	CodePlain   string   `json:"codePlain"`
	Narration   string   `json:"narration"`
	AudioChunks []string `json:"audioChunks"`
}

// GetJob reads a job from S3 and returns prompt, result, and segments with audio as base64.
func (c *Client) GetJob(ctx context.Context, jobID string) (*JobLoaded, error) {
	if c.bucket == "" {
		return nil, fmt.Errorf("S3 not configured")
	}
	prefix := jobID + "/"

	// prompt.txt
	prompt, err := c.get(ctx, prefix+"prompt.txt")
	if err != nil {
		return nil, err
	}

	// result.json
	resultBody, err := c.get(ctx, prefix+"result.json")
	if err != nil {
		return nil, err
	}
	var result ResultStored
	if err := json.Unmarshal(resultBody, &result); err != nil {
		return nil, err
	}

	out := &JobLoaded{
		Prompt:        string(prompt),
		RawJSON:       result.RawJSON,
		FullCode:      result.FullCode,
		CSS:           result.CSS,
		Title:         result.Title,
		NarrationLang: result.NarrationLang,
		StoryHTML:     result.StoryHTML,
		Segments:      make([]SegmentWithAudio, len(result.Segments)),
	}
	for i := range result.Segments {
		out.Segments[i] = SegmentWithAudio{
			Code:      result.Segments[i].Code,
			CodePlain: result.Segments[i].CodePlain,
			Narration: result.Segments[i].Narration,
		}
		// Load N.pcm and encode as base64
		pcm, err := c.get(ctx, prefix+strconv.Itoa(i)+".pcm")
		if err == nil && len(pcm) > 0 {
			out.Segments[i].AudioChunks = []string{base64.StdEncoding.EncodeToString(pcm)}
		}
	}
	return out, nil
}

func (c *Client) put(ctx context.Context, key string, body []byte) error {
	if c.client == nil {
		return nil
	}
	_, err := c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(body),
	})
	return err
}

func (c *Client) get(ctx context.Context, key string) ([]byte, error) {
	if c.client == nil {
		return nil, fmt.Errorf("S3 not configured")
	}
	resp, err := c.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// ListRecent returns the most recent limit jobs from S3, ordered newest-first.
// Job IDs are UUIDv7, so lexicographic order equals chronological order; we page
// through all common prefixes and take the last `limit` entries.
// result.json for each job is fetched concurrently to retrieve the title.
func (c *Client) ListRecent(ctx context.Context, limit int) ([]ports.JobMeta, error) {
	if c.bucket == "" || c.client == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}

	// Collect all top-level common prefixes (one per job directory).
	var prefixes []string
	var contToken *string
	for {
		resp, err := c.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(c.bucket),
			Delimiter:         aws.String("/"),
			MaxKeys:           aws.Int32(1000),
			ContinuationToken: contToken,
		})
		if err != nil {
			return nil, fmt.Errorf("list objects: %w", err)
		}
		for _, p := range resp.CommonPrefixes {
			if p.Prefix != nil {
				prefixes = append(prefixes, *p.Prefix)
			}
		}
		if resp.IsTruncated == nil || !*resp.IsTruncated || resp.NextContinuationToken == nil {
			break
		}
		contToken = resp.NextContinuationToken
	}

	// Take the last `limit` entries (most recent, since UUIDv7 sorts chronologically).
	if len(prefixes) > limit {
		prefixes = prefixes[len(prefixes)-limit:]
	}

	// Reverse so the newest appears first.
	for i, j := 0, len(prefixes)-1; i < j; i, j = i+1, j-1 {
		prefixes[i], prefixes[j] = prefixes[j], prefixes[i]
	}

	// Fetch result.json for each prefix concurrently, preserving order.
	type slot struct {
		meta ports.JobMeta
		ok   bool
	}
	slots := make([]slot, len(prefixes))
	var wg sync.WaitGroup
	for i, prefix := range prefixes {
		i, prefix := i, prefix
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := strings.TrimSuffix(prefix, "/")
			body, err := c.get(ctx, prefix+"result.json")
			if err != nil {
				log.Debug().Err(err).Str("prefix", prefix).Msg("list recent: get result.json")
				return
			}
			var res ResultStored
			if err := json.Unmarshal(body, &res); err != nil {
				return
			}
			title := res.Title
			if title == "" {
				title = id
			}
			slots[i] = slot{meta: ports.JobMeta{ID: id, Title: title}, ok: true}
		}()
	}
	wg.Wait()

	out := make([]ports.JobMeta, 0, len(slots))
	for _, s := range slots {
		if s.ok {
			out = append(out, s.meta)
		}
	}
	return out, nil
}
