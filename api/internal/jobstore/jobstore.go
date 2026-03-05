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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/rs/zerolog/log"
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
	if opts.Endpoint != "" {
		endpoint := strings.TrimSuffix(opts.Endpoint, "/")
		cfg.EndpointResolverWithOptions = aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{URL: endpoint}, nil
		})
	}
	return &Client{
		bucket: opts.Bucket,
		client: s3.NewFromConfig(cfg),
	}, nil
}

// SegmentStored is the shape we store per segment (code HTML, codePlain, narration).
// Audio is stored as separate 0.pcm, 1.pcm, ... files.
type SegmentStored struct {
	Code      string `json:"code"`
	CodePlain string `json:"codePlain"`
	Narration string `json:"narration"`
}

// ResultStored is the JSON we store as result.json (full code + segments without audio).
type ResultStored struct {
	RawJSON       string           `json:"rawJson"`
	FullCode      string           `json:"fullCode"`
	FullCodePlain string           `json:"fullCodePlain"`
	Segments      []SegmentStored  `json:"segments"`
}

// UploadJob writes prompt, result JSON, and per-segment PCM files under jobID/.
func (c *Client) UploadJob(ctx context.Context, jobID, prompt, rawJSON, fullCode, fullCodePlain string, segments []SegmentStored, segmentAudio [][]byte) error {
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
		Segments:      segments,
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
	Prompt   string   `json:"prompt"`
	RawJSON  string   `json:"rawJson"`
	FullCode string   `json:"fullCode"`
	Segments []SegmentWithAudio `json:"segments"`
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
		Prompt:   string(prompt),
		RawJSON:  result.RawJSON,
		FullCode: result.FullCode,
		Segments: make([]SegmentWithAudio, len(result.Segments)),
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
