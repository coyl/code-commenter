package gemini

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/genai"

)

// Client wraps Gemini 3.1 for task spec, CSS, code, and diff generation.
type Client struct {
	client *genai.Client
	model  string
}

// NewClient creates a Gemini client using the given API key and model.
func NewClient(ctx context.Context, apiKey, model string) (*Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("gemini API key is required")
	}
	c, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("genai NewClient: %w", err)
	}
	if model == "" {
		model = "gemini-3-flash-preview"
	}
	return &Client{client: c, model: model}, nil
}

// Close is a no-op for the GenAI client (SDK does not require closing).
func (c *Client) Close() error {
	return nil
}

// GenerateTaskSpec asks Gemini to turn a task description into a structured spec and optional narration script.
func (c *Client) GenerateTaskSpec(ctx context.Context, task, language string) (spec, narrationScript string, err error) {
	start := time.Now()
	defer func() {
		ev := log.Info().Str("op", "GenerateTaskSpec").Dur("dur", time.Since(start))
		if err != nil {
			ev = log.Error().Err(err).Str("op", "GenerateTaskSpec").Dur("dur", time.Since(start))
		}
		ev.Msg("llm request")
	}()
	prompt := fmt.Sprintf(`You are a coding assistant. The user will describe a coding task. Output a short structured spec (what to build) and an optional narration script for a voiceover that explains the code in 2-4 sentences.

Task: %s
Language: %s

Respond in this exact format, no other text:
SPEC:
<one or two sentence spec>

NARRATION:
<2-4 sentences for voiceover>`, task, language)

	result, err := c.client.Models.GenerateContent(ctx, c.model, []*genai.Content{
		{Parts: []*genai.Part{{Text: prompt}}},
	}, nil)
	if err != nil {
		return "", "", err
	}
	text := extractText(result)
	spec, narrationScript = parseSpecAndNarration(text)
	return spec, narrationScript, nil
}

// GenerateCSS produces a single block of CSS for the code view/theme/layout and syntax highlighting.
func (c *Client) GenerateCSS(ctx context.Context, spec, language string) (css string, err error) {
	prompt := fmt.Sprintf(`Generate a single block of CSS for a code viewer page. The page shows code in a monospace editor with syntax highlighting. Language: %s. Context: %s

Output only valid CSS, no markdown code fences. Include:
- A container for the code view (e.g. #code-view or .code-view): background, border, padding
- Base code style: font-family monospace, font-size, line height, default text color
- Syntax highlighting classes for tokens (use these exact class names so the frontend can style them):
  .token-keyword   { color for keywords: function, const, if, return, etc. }
  .token-string    { color for string literals }
  .token-comment   { color for line and block comments }
  .token-number    { color for numeric literals }
  .token-function  { color for function/method names }
  .token-operator  { color for operators: +, -, =, etc. }
  .token-punctuation { color for brackets, commas, semicolons }
  .token-variable  { color for variables and identifiers }
Pick a cohesive color scheme (e.g. dark background with cyan/green/amber accents).`, language, spec)

	start := time.Now()
	result, err := c.client.Models.GenerateContent(ctx, c.model, []*genai.Content{
		{Parts: []*genai.Part{{Text: prompt}}},
	}, nil)
	if err != nil {
		log.Error().Err(err).Str("op", "GenerateCSS").Dur("dur", time.Since(start)).Msg("llm request")
		return "", err
	}
	log.Info().Str("op", "GenerateCSS").Dur("dur", time.Since(start)).Msg("llm request")
	return strings.TrimSpace(cleanCodeBlock(extractText(result))), nil
}

// GenerateCode produces full source code in the requested language.
func (c *Client) GenerateCode(ctx context.Context, spec, language string) (code string, err error) {
	prompt := fmt.Sprintf(`Generate full source code that fulfills this spec. Language: %s

Spec: %s

Output only the code, no markdown code fences or explanation.`, language, spec)

	start := time.Now()
	result, err := c.client.Models.GenerateContent(ctx, c.model, []*genai.Content{
		{Parts: []*genai.Part{{Text: prompt}}},
	}, nil)
	if err != nil {
		log.Error().Err(err).Str("op", "GenerateCode").Dur("dur", time.Since(start)).Msg("llm request")
		return "", err
	}
	log.Info().Str("op", "GenerateCode").Dur("dur", time.Since(start)).Msg("llm request")
	return strings.TrimSpace(cleanCodeBlock(extractText(result))), nil
}

// CodeSegment is one logical part of the code (plain text) with its narration for voiceover.
// Syntax highlighting is applied server-side; JSON uses short keys (c=code, n=narration).
type CodeSegment struct {
	Code      string `json:"c"`
	Narration string `json:"n"`
}

// segmentsSchema returns the JSON schema for structured output: array of { c: code, n: narration }.
func segmentsSchema() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeArray,
		Items: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"c": {
					Type:        genai.TypeString,
					Description: "Plain source code for this segment (imports, one function, or a logical block). Preserve exact formatting, indentation, and newlines.",
				},
				"n": {
					Type:        genai.TypeString,
					Description: "One or two short sentences for a voiceover explaining this segment.",
				},
			},
			Required: []string{"c", "n"},
		},
	}
}

// GenerateCodeSegments returns code split into logical segments (plain code + narration) and the raw JSON from the LLM.
// Highlighting is done server-side with Chroma; the LLM only outputs correct source code.
func (c *Client) GenerateCodeSegments(ctx context.Context, spec, language string) ([]CodeSegment, string, error) {
	prompt := fmt.Sprintf(`Generate source code that fulfills this spec. Language: %s

Spec: %s

Split the code into 3–8 logical segments (e.g. imports, then each function or type, or logical blocks).
Output valid, well-formatted source code. Syntax highlighting will be applied automatically.

Each segment has:
- "c": plain source code for this segment (exact characters, correct syntax, proper indentation and newlines).
- "n": one or two short sentences for a voiceover explaining this segment.

Rules:
- Output only valid %s code. No markdown, no code fences.
- Preserve exact indentation (tabs or spaces) and newlines.
- Each segment should be self-contained (e.g. one or more complete declarations).`, language, spec, language)

	start := time.Now()
	cfg := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		ResponseSchema:   segmentsSchema(),
	}
	result, err := c.client.Models.GenerateContent(ctx, c.model, []*genai.Content{
		{Parts: []*genai.Part{{Text: prompt}}},
	}, cfg)
	if err != nil {
		log.Error().Err(err).Str("op", "GenerateCodeSegments").Dur("dur", time.Since(start)).Msg("llm request")
		return nil, "", err
	}
	log.Info().Str("op", "GenerateCodeSegments").Dur("dur", time.Since(start)).Msg("llm request")
	text := extractText(result)
	segments, err := parseSegmentsJSON(text)
	if err != nil {
		return nil, text, err
	}
	return segments, text, nil
}

// GenerateWrappingNarration returns a short closing voiceover that summarizes what was built (no code, narration only).
func (c *Client) GenerateWrappingNarration(ctx context.Context, spec, language string) (string, error) {
	prompt := fmt.Sprintf(`Summarize in 2 to 4 short sentences what was built, for a closing voiceover. No code, no markdown.

Spec: %s
Language: %s

Output only the narration text, nothing else.`, spec, language)
	start := time.Now()
	result, err := c.client.Models.GenerateContent(ctx, c.model, []*genai.Content{
		{Parts: []*genai.Part{{Text: prompt}}},
	}, nil)
	if err != nil {
		log.Error().Err(err).Str("op", "GenerateWrappingNarration").Dur("dur", time.Since(start)).Msg("llm request")
		return "", err
	}
	log.Info().Str("op", "GenerateWrappingNarration").Dur("dur", time.Since(start)).Msg("llm request")
	return strings.TrimSpace(extractText(result)), nil
}

func parseSegmentsJSON(text string) ([]CodeSegment, error) {
	text = strings.TrimSpace(text)
	// The schema returns a top-level array.
	var segments []CodeSegment
	if err := json.Unmarshal([]byte(text), &segments); err != nil {
		return nil, fmt.Errorf("parse segments JSON: %w\nraw: %.500s", err, text)
	}
	return segments, nil
}

// GenerateCodeStream yields code text chunks via the given callback. Each chunk is a delta; full code can be built by concatenation.
func (c *Client) GenerateCodeStream(ctx context.Context, spec, language string, yield func(chunk string) error) (fullCode string, err error) {
	start := time.Now()
	defer func() {
		ev := log.Info().Str("op", "GenerateCodeStream").Dur("dur", time.Since(start))
		if err != nil {
			ev = log.Error().Err(err).Str("op", "GenerateCodeStream").Dur("dur", time.Since(start))
		}
		ev.Msg("llm request")
	}()
	prompt := fmt.Sprintf(`Generate full source code that fulfills this spec. Language: %s

Spec: %s

Output only the code, no markdown code fences or explanation.`, language, spec)

	var full strings.Builder
	for resp, err := range c.client.Models.GenerateContentStream(ctx, c.model, []*genai.Content{
		{Parts: []*genai.Part{{Text: prompt}}},
	}, nil) {
		if err != nil {
			return full.String(), err
		}
		chunk := extractText(resp)
		if chunk != "" {
			full.WriteString(chunk)
			if yield != nil && yield(chunk) != nil {
				return full.String(), nil
			}
		}
	}
	return strings.TrimSpace(cleanCodeBlock(full.String())), nil
}

// GenerateAudioStream uses REST TTS (response_modalities: ["audio"]) to generate speech from script and yields base64-encoded PCM chunks (same format as Live API for the frontend).
func (c *Client) GenerateAudioStream(ctx context.Context, ttsModel, script string, yield func(base64Chunk string) error) (err error) {
	start := time.Now()
	defer func() {
		ev := log.Info().Str("op", "GenerateAudioStream").Str("model", ttsModel).Dur("dur", time.Since(start))
		if err != nil {
			ev = log.Error().Err(err).Str("op", "GenerateAudioStream").Str("model", ttsModel).Dur("dur", time.Since(start))
		}
		ev.Msg("llm request")
	}()
	if ttsModel == "" {
		ttsModel = "gemini-2.5-flash-preview-tts"
	}
	temp := float32(1.0)
	cfg := &genai.GenerateContentConfig{
		Temperature:        &temp,
		ResponseModalities: []string{"audio"},
		SpeechConfig: &genai.SpeechConfig{
			VoiceConfig: &genai.VoiceConfig{
				PrebuiltVoiceConfig: &genai.PrebuiltVoiceConfig{VoiceName: "Puck"},
			},
		},
	}
	contents := []*genai.Content{
		{Role: "user", Parts: []*genai.Part{{Text: script}}},
	}
	for resp, err := range c.client.Models.GenerateContentStream(ctx, ttsModel, contents, cfg) {
		if err != nil {
			return err
		}
		for _, cand := range resp.Candidates {
			if cand.Content == nil {
				continue
			}
			for _, p := range cand.Content.Parts {
				if p.InlineData != nil && len(p.InlineData.Data) > 0 {
					b64 := base64.StdEncoding.EncodeToString(p.InlineData.Data)
					if yield != nil && yield(b64) != nil {
						return nil
					}
				}
			}
		}
	}
	return nil
}

// GenerateChange produces updated CSS, full new code, and a code diff given current state and user request.
func (c *Client) GenerateChange(ctx context.Context, currentCSS, currentCode, userMessage, language string) (newCSS, newCode, unifiedDiff string, err error) {
	prompt := fmt.Sprintf(`You are a coding assistant. The user wants to change the current code/CSS.

Current CSS (full block):
---CSS---
%s
---END CSS---

Current code (full source):
---CODE---
%s
---END CODE---

User request: %s
Language: %s

Respond in this exact format only (no other text):
---NEW CSS---
<full updated CSS block>
---END NEW CSS---
---NEW CODE---
<full new source code after applying the change>
---END NEW CODE---
---UNIFIED DIFF---
<unified diff for the code change, e.g. diff -u style>
---END UNIFIED DIFF---`, currentCSS, currentCode, userMessage, language)

	start := time.Now()
	result, err := c.client.Models.GenerateContent(ctx, c.model, []*genai.Content{
		{Parts: []*genai.Part{{Text: prompt}}},
	}, nil)
	if err != nil {
		log.Error().Err(err).Str("op", "GenerateChange").Dur("dur", time.Since(start)).Msg("llm request")
		return "", "", "", err
	}
	log.Info().Str("op", "GenerateChange").Dur("dur", time.Since(start)).Msg("llm request")
	text := extractText(result)
	newCSS, newCode, unifiedDiff = parseChangeResponse(text)
	return newCSS, newCode, unifiedDiff, nil
}

func extractText(resp *genai.GenerateContentResponse) string {
	if resp == nil || len(resp.Candidates) == 0 {
		return ""
	}
	c := resp.Candidates[0]
	if c.Content == nil {
		return ""
	}
	var b strings.Builder
	for _, p := range c.Content.Parts {
		if p.Text != "" {
			b.WriteString(p.Text)
		}
	}
	return b.String()
}

func cleanCodeBlock(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		lines := strings.SplitN(s, "\n", 2)
		if len(lines) == 2 {
			s = lines[1]
		}
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimSpace(s)
	}
	return s
}

func parseSpecAndNarration(text string) (spec, narration string) {
	text = strings.TrimSpace(text)
	specIdx := strings.Index(text, "SPEC:")
	narIdx := strings.Index(text, "NARRATION:")
	if specIdx >= 0 && narIdx > specIdx {
		spec = strings.TrimSpace(text[specIdx+5 : narIdx])
		narration = strings.TrimSpace(text[narIdx+10:])
		return spec, narration
	}
	if specIdx >= 0 {
		spec = strings.TrimSpace(text[specIdx+5:])
	}
	return spec, text
}

func parseChangeResponse(text string) (newCSS, newCode, unifiedDiff string) {
	text = strings.TrimSpace(text)
	cssStart := strings.Index(text, "---NEW CSS---")
	cssEnd := strings.Index(text, "---END NEW CSS---")
	codeStart := strings.Index(text, "---NEW CODE---")
	codeEnd := strings.Index(text, "---END NEW CODE---")
	diffStart := strings.Index(text, "---UNIFIED DIFF---")
	diffEnd := strings.Index(text, "---END UNIFIED DIFF---")
	if cssStart >= 0 && cssEnd > cssStart {
		newCSS = strings.TrimSpace(text[cssStart+12 : cssEnd])
	}
	if codeStart >= 0 && codeEnd > codeStart {
		newCode = strings.TrimSpace(text[codeStart+12 : codeEnd])
		newCode = cleanCodeBlock(newCode)
	}
	if diffStart >= 0 && diffEnd > diffStart {
		unifiedDiff = strings.TrimSpace(text[diffStart+17 : diffEnd])
	}
	return newCSS, newCode, unifiedDiff
}
