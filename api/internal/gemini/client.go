package gemini

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	_ "image/jpeg"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"google.golang.org/genai"
)

// TTSSampleRate is the sample rate of PCM output from Gemini TTS (24 kHz, 16-bit LE).
const TTSSampleRate = 24000

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

// defaultSafetySettings returns safety settings that block medium-and-above harm in standard categories.
// Used for all generation requests to filter unsafe content.
func defaultSafetySettings() []*genai.SafetySetting {
	return []*genai.SafetySetting{
		{Category: genai.HarmCategoryHarassment, Threshold: genai.HarmBlockThresholdBlockMediumAndAbove},
		{Category: genai.HarmCategoryHateSpeech, Threshold: genai.HarmBlockThresholdBlockMediumAndAbove},
		{Category: genai.HarmCategorySexuallyExplicit, Threshold: genai.HarmBlockThresholdBlockMediumAndAbove},
		{Category: genai.HarmCategoryDangerousContent, Threshold: genai.HarmBlockThresholdBlockMediumAndAbove},
	}
}

// googleSearchTool returns a Tool that enables Google Search grounding for the request.
func googleSearchTool() []*genai.Tool {
	return []*genai.Tool{{GoogleSearch: &genai.GoogleSearch{}}}
}

// narrationLanguageLabel returns a display name for the narration language (for LLM prompts).
func narrationLanguageLabel(lang string) string {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "german":
		return "German"
	case "spanish":
		return "Spanish"
	case "italian":
		return "Italian"
	case "chinese":
		return "Chinese (Simplified)"
	default:
		return "English"
	}
}

// GenerateTaskSpec asks Gemini to turn a task description into a structured spec and optional narration script.
func (c *Client) GenerateTaskSpec(ctx context.Context, task, language, narrationLang string) (spec, narrationScript string, err error) {
	start := time.Now()
	defer func() {
		ev := log.Info().Str("op", "GenerateTaskSpec").Dur("dur", time.Since(start))
		if err != nil {
			ev = log.Error().Err(err).Str("op", "GenerateTaskSpec").Dur("dur", time.Since(start))
		}
		ev.Msg("llm request")
	}()
	narrationLabel := narrationLanguageLabel(narrationLang)
	prompt := fmt.Sprintf(`You are a coding assistant. The user will describe a coding task. Output a short structured spec (what to build) and an optional narration script for a voiceover that explains the code in 2-4 sentences. Write the NARRATION in %s.

Task: %s
Language: %s

Respond in this exact format, no other text:
SPEC:
<one or two sentence spec>

NARRATION:
<2-4 sentences for voiceover in %s>`, narrationLabel, task, language, narrationLabel)

	cfg := &genai.GenerateContentConfig{
		SafetySettings: defaultSafetySettings(),
		Tools:          googleSearchTool(),
	}
	result, err := c.client.Models.GenerateContent(ctx, c.model, []*genai.Content{
		{Parts: []*genai.Part{{Text: prompt}}},
	}, cfg)
	if err != nil {
		return "", "", err
	}
	text := extractText(result)
	spec, narrationScript = parseSpecAndNarration(text)
	return spec, narrationScript, nil
}

// GenerateCSS produces a single block of CSS for the code view/theme/layout and syntax highlighting.
func (c *Client) GenerateCSS(ctx context.Context, spec, language string) (css string, err error) {
	langHint := strings.TrimSpace(language)
	if langHint == "" {
		langHint = "any (language-agnostic)"
	}
	prompt := fmt.Sprintf(`Generate a single block of CSS for a code viewer page. The page shows code in a monospace editor with syntax highlighting. Language: %s. Context: %s

CRITICAL: Every selector MUST be scoped under #code-view so it only affects the code block. Use exactly:
#code-view { container: background, border, padding, font-family monospace, font-size, line-height, default text color }
#code-view .token-keyword   { color for keywords: function, const, if, return, etc. }
#code-view .token-string   { color for string literals }
#code-view .token-comment  { color for line and block comments }
#code-view .token-number   { color for numeric literals }
#code-view .token-function  { color for function/method names }
#code-view .token-operator  { color for operators: +, -, =, etc. }
#code-view .token-punctuation { color for brackets, commas, semicolons }
#code-view .token-variable { color for variables and identifiers }

You MUST define each of the eight .token-* classes with a visibly different color (e.g. keyword=cyan, string=green, comment=gray italic, number=amber, function=purple, operator=slate, punctuation=slate, variable=white). Do not use a single color for all tokens.
Output only valid CSS, no markdown code fences. Pick a cohesive color scheme (e.g. dark background with cyan/green/amber accents).`, langHint, spec)

	start := time.Now()
	cfg := &genai.GenerateContentConfig{SafetySettings: defaultSafetySettings()}
	result, err := c.client.Models.GenerateContent(ctx, c.model, []*genai.Content{
		{Parts: []*genai.Part{{Text: prompt}}},
	}, cfg)
	if err != nil {
		log.Error().Err(err).Str("op", "GenerateCSS").Dur("dur", time.Since(start)).Msg("llm request")
		return "", err
	}
	log.Info().Str("op", "GenerateCSS").Dur("dur", time.Since(start)).Msg("llm request")
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
					Description: "Plain source code for this segment: a few lines or one logical step (e.g. imports, one small function, or one part of a large function). Preserve exact formatting, indentation, and newlines.",
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
func (c *Client) GenerateCodeSegments(ctx context.Context, spec, language, narrationLang string) ([]CodeSegment, string, error) {
	narrationLabel := narrationLanguageLabel(narrationLang)
	prompt := fmt.Sprintf(`Generate source code that fulfills this spec. Language: %s

Spec: %s

Split the code into at most 9 segments so each segment is small enough for a clear, focused narration.
- Imports and top-level declarations: one segment each (or grouped if very small).
- Small functions (e.g. under ~8 lines): one segment per function is fine.
- Large functions: split into multiple segments (e.g. signature and setup, then main logic in steps, then return/cleanup) so the narration can explain details (parameters, loops, conditions, return value). Do not put an entire long function in one segment. If you would need more than 9 segments, group some logical blocks together to stay within 9.
Output valid, well-formatted source code. Syntax highlighting will be applied automatically.

Each segment has:
- "c": plain source code for this segment (exact characters, correct syntax, proper indentation and newlines).
- "n": one or two short sentences for a voiceover explaining this segment. Write the narration in %s.

Rules:
- Output only valid %s code. No markdown, no code fences.
- Preserve exact indentation (tabs or spaces) and newlines.
- Each segment should be self-contained (complete statements or a clear logical step).`, language, spec, narrationLabel, language)

	start := time.Now()
	cfg := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		ResponseSchema:   segmentsSchema(),
		SafetySettings:   defaultSafetySettings(),
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

// FormatAndSegmentCode takes user-provided code and returns it with only indentation/newlines
// beautified, split into logical segments with narration. The code logic and text must be preserved as-is.
// The LLM infers the programming language from the code; no language is passed.
func (c *Client) FormatAndSegmentCode(ctx context.Context, code, narrationLang string) ([]CodeSegment, string, error) {
	narrationLabel := narrationLanguageLabel(narrationLang)
	prompt := fmt.Sprintf(`You are given raw source code. Your job is to:
1. Preserve the code EXACTLY as-is: do not change any logic, identifiers, strings, or behavior.
2. Only beautify: normalize indentation (consistent spaces or tabs) and newlines (trim trailing, consistent line endings).
3. Split the result into at most 9 segments so each segment is small enough for a clear, focused narration.
   - Imports and top-level declarations: one segment each (or grouped if very small).
   - Small functions (e.g. under ~8 lines): one segment per function is fine.
   - Large functions: split into multiple segments (e.g. signature and setup, main logic in steps, return/cleanup) so the narration can explain details. Do not put an entire long function in one segment. If you would need more than 9 segments, group some logical blocks together to stay within 9.

Infer the programming language from the code (e.g. JavaScript, Python, Go)

---CODE---
%s
---END CODE---

Output a JSON array of segments. Each segment has:
- "c": plain source code for this segment. Must be exactly the code from above with only indentation/newlines possibly normalized. No changes to logic or text.
- "n": one or two short sentences for a voiceover explaining this segment. Write the narration in %s.

Rules:
- Do not add, remove, or alter any code logic. Only fix indentation and newlines.
- Each segment should be self-contained (complete statements or a clear logical step).
- Output only the JSON array, no other text.`, code, narrationLabel)

	start := time.Now()
	cfg := &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		ResponseSchema:   segmentsSchema(),
		SafetySettings:   defaultSafetySettings(),
	}
	result, err := c.client.Models.GenerateContent(ctx, c.model, []*genai.Content{
		{Parts: []*genai.Part{{Text: prompt}}},
	}, cfg)
	if err != nil {
		log.Error().Err(err).Str("op", "FormatAndSegmentCode").Dur("dur", time.Since(start)).Msg("llm request")
		return nil, "", err
	}
	log.Info().Str("op", "FormatAndSegmentCode").Dur("dur", time.Since(start)).Msg("llm request")
	text := extractText(result)
	segments, err := parseSegmentsJSON(text)
	if err != nil {
		return nil, text, err
	}
	return segments, text, nil
}

// GenerateWrappingNarration returns a short closing voiceover that summarizes what was built (no code, narration only).
func (c *Client) GenerateWrappingNarration(ctx context.Context, spec, language, narrationLang string) (string, error) {
	narrationLabel := narrationLanguageLabel(narrationLang)
	prompt := fmt.Sprintf(`Write a closing voiceover (3 to 5 short sentences) in %s. Do all of the following:
1. Briefly summarize what was built.
2. Point out the most important or interesting part of the code (e.g. the core logic, main function, or key pattern).
3. Explain that part in one or two short sentences so the listener understands why it matters.

No code snippets, no markdown. Output only the narration text, nothing else.

Spec: %s
Language: %s`, narrationLabel, spec, language)
	start := time.Now()
	cfg := &genai.GenerateContentConfig{SafetySettings: defaultSafetySettings()}
	result, err := c.client.Models.GenerateContent(ctx, c.model, []*genai.Content{
		{Parts: []*genai.Part{{Text: prompt}}},
	}, cfg)
	if err != nil {
		log.Error().Err(err).Str("op", "GenerateWrappingNarration").Dur("dur", time.Since(start)).Msg("llm request")
		return "", err
	}
	log.Info().Str("op", "GenerateWrappingNarration").Dur("dur", time.Since(start)).Msg("llm request")
	return strings.TrimSpace(extractText(result)), nil
}

// GenerateWrappingNarrationForUserCode returns a short closing voiceover for user-pasted code, in the requested narration language.
func (c *Client) GenerateWrappingNarrationForUserCode(ctx context.Context, segmentNarrationsSummary, narrationLang string) (string, error) {
	narrationLabel := narrationLanguageLabel(narrationLang)
	prompt := fmt.Sprintf(`The following are short descriptions of segments of user-provided code (for a code walkthrough):

%s

Write a closing voiceover (3 to 5 short sentences) in %s. Do all of the following:
1. Briefly summarize what the code does.
2. Point out the most important or interesting part (e.g. the core logic, main function, or key pattern).
3. Explain that part in one or two short sentences so the listener understands why it matters.

No code snippets, no markdown. Output only the narration text, nothing else.`, segmentNarrationsSummary, narrationLabel)
	start := time.Now()
	cfg := &genai.GenerateContentConfig{SafetySettings: defaultSafetySettings()}
	result, err := c.client.Models.GenerateContent(ctx, c.model, []*genai.Content{
		{Parts: []*genai.Part{{Text: prompt}}},
	}, cfg)
	if err != nil {
		log.Error().Err(err).Str("op", "GenerateWrappingNarrationForUserCode").Dur("dur", time.Since(start)).Msg("llm request")
		return "", err
	}
	log.Info().Str("op", "GenerateWrappingNarrationForUserCode").Dur("dur", time.Since(start)).Msg("llm request")
	return strings.TrimSpace(extractText(result)), nil
}

// GenerateStory returns an HTML article body describing the problem and its solution.
// The returned HTML contains the marker {{EMBED_PLAYER}} exactly once, positioned mid-article so
// the frontend can inject an embed iframe there. The article text is written in narrationLang.
func (c *Client) GenerateStory(ctx context.Context, title, spec, language, narrationLang, segmentNarrations string) (storyHTML string, err error) {
	start := time.Now()
	defer func() {
		ev := log.Info().Str("op", "GenerateStory").Dur("dur", time.Since(start))
		if err != nil {
			ev = log.Error().Err(err).Str("op", "GenerateStory").Dur("dur", time.Since(start))
		}
		ev.Msg("llm request")
	}()

	narrationLabel := narrationLanguageLabel(narrationLang)
	prompt := fmt.Sprintf(`You are a technical writer. Write a short, engaging article about the coding problem below and how it was solved. The article will be published on a developer blog and will have an interactive code player embedded in the middle.

Title: %s
Programming language: %s
Problem/Spec: %s
How it was solved (segment narrations):
%s

Rules:
- Write the ENTIRE article in %s (same language as the voiceover/narration).
- Write ONLY the HTML body content — no <html>, <head>, or <body> tags, no CSS, no <script>.
- Use only these tags: <h1>, <h2>, <p>, <ul>, <li>, <strong>, <em>, <code>.
- Structure: a short intro (1-2 paragraphs) that sets up the problem, then the exact text {{EMBED_PLAYER}} on its own line (this is where the interactive player will be inserted), then a conclusion (1-2 paragraphs) that summarises what was built and what to take away.
- Keep it concise: roughly 200-350 words total.
- Do NOT include the actual source code in the article body; the player shows it.
- Output only the HTML, no markdown code fences, no explanation.`, title, language, spec, segmentNarrations, narrationLabel)

	cfg := &genai.GenerateContentConfig{
		SafetySettings: defaultSafetySettings(),
		Tools:          googleSearchTool(),
	}
	result, err := c.client.Models.GenerateContent(ctx, c.model, []*genai.Content{
		{Parts: []*genai.Part{{Text: prompt}}},
	}, cfg)
	if err != nil {
		return "", err
	}
	raw := strings.TrimSpace(extractText(result))
	raw = cleanCodeBlock(raw)
	// Ensure the marker is present; append a fallback section if the LLM omitted it.
	if !strings.Contains(raw, "{{EMBED_PLAYER}}") {
		raw = raw + "\n<p>{{EMBED_PLAYER}}</p>"
	}
	return raw, nil
}

const imageModel = "gemini-3.1-flash-image-preview"

func (c *Client) GenerateImages(ctx context.Context, title, spec, language, segmentNarrations string) (preview, illustration string, err error) {
	start := time.Now()
	defer func() {
		ev := log.Info().Str("op", "GenerateImages").Dur("dur", time.Since(start))
		if err != nil {
			ev = log.Error().Err(err).Str("op", "GenerateImages").Dur("dur", time.Since(start))
		}
		ev.Msg("llm request")
	}()

	previewPrompt := fmt.Sprintf(`Create a clean, simple YouTube-style video thumbnail for a developer coding tutorial.
Title: %s
Language: %s
Topic: %s

Rules:
- Dark background (like a dark IDE theme).
- Show a bold, clean visual concept for the coding topic — NOT actual code text.
- Use simple geometric shapes, icons, or abstract design elements that represent the concept.
- Bold, high-contrast typography for a short (3-5 word) version of the title.
- Professional, polished look. No clutter. No actual source code text.
Output only the image.`, title, language, spec)

	illustrationPrompt := fmt.Sprintf(`Create a clean technical diagram or conceptual scheme for this coding solution.
Title: %s
Language: %s
Approach (narration summary): %s

Rules:
- Dark background with light-colored elements.
- A clear flowchart, architecture diagram, or concept map — whichever best fits the approach.
- Simple labeled shapes and arrows. No actual source code.
- Clean, minimal, educational layout easy to read at a glance.
Output only the diagram image.`, title, language, segmentNarrations)

	cfg := &genai.GenerateContentConfig{
		ResponseModalities: []string{"IMAGE"},
		ImageConfig: &genai.ImageConfig{
			AspectRatio: "16:9",
		},
		SafetySettings: defaultSafetySettings(),
	}

	type result struct {
		b64 string
		err error
	}
	var wg sync.WaitGroup
	prevCh := make(chan result, 1)
	illustCh := make(chan result, 1)

	generate := func(prompt string, ch chan<- result) {
		defer wg.Done()
		resp, e := c.client.Models.GenerateContent(ctx, imageModel, []*genai.Content{
			{Parts: []*genai.Part{{Text: prompt}}},
		}, cfg)
		if e != nil {
			ch <- result{err: e}
			return
		}
		raw := extractImageBytes(resp)
		if len(raw) == 0 {
			ch <- result{err: fmt.Errorf("no image data returned")}
			return
		}
		ch <- result{b64: base64.StdEncoding.EncodeToString(raw)}
	}

	wg.Add(2)
	go generate(previewPrompt, prevCh)
	go generate(illustrationPrompt, illustCh)
	wg.Wait()

	prevRes := <-prevCh
	illustRes := <-illustCh

	if prevRes.err != nil && illustRes.err != nil {
		err = fmt.Errorf("preview: %w; illustration: %v", prevRes.err, illustRes.err)
		return "", "", err
	}
	if prevRes.err != nil {
		log.Warn().Err(prevRes.err).Msg("preview image generation failed")
	}
	if illustRes.err != nil {
		log.Warn().Err(illustRes.err).Msg("illustration image generation failed")
	}
	return prevRes.b64, illustRes.b64, nil
}

// extractImageBytes pulls the first inline image bytes from a GenerateContent response.
func extractImageBytes(resp *genai.GenerateContentResponse) []byte {
	if resp == nil || len(resp.Candidates) == 0 {
		return nil
	}
	c := resp.Candidates[0]
	if c.Content == nil {
		return nil
	}
	for _, p := range c.Content.Parts {
		if p.InlineData != nil && len(p.InlineData.Data) > 0 {
			return p.InlineData.Data
		}
	}
	return nil
}

// GenerateTitle returns a short title for a job (from spec and prompt/task, or from segment narrations for user code), written in narrationLang.
func (c *Client) GenerateTitle(ctx context.Context, spec, prompt, narrationLang string) (string, error) {
	if strings.TrimSpace(prompt) == "" {
		prompt = spec
	}
	if strings.TrimSpace(prompt) == "" {
		// Default fallback in English; caller can pass narrationLang to get a localized default if needed.
		return "Code walkthrough", nil
	}
	narrationLabel := narrationLanguageLabel(narrationLang)
	promptText := fmt.Sprintf(`Given the following (a coding task description or a walkthrough of what the code does), output a single short title (3–8 words) that captures the essence. Write the title in %s. Examples (if English): "React counter with hooks", "Python binary search", "Go HTTP server with middleware". Do not use generic phrases like "Analysis of user provided code". No quotes, no period.

%s

Output only the title, nothing else.`, narrationLabel, prompt)
	start := time.Now()
	cfg := &genai.GenerateContentConfig{SafetySettings: defaultSafetySettings()}
	result, err := c.client.Models.GenerateContent(ctx, c.model, []*genai.Content{
		{Parts: []*genai.Part{{Text: promptText}}},
	}, cfg)
	if err != nil {
		log.Error().Err(err).Str("op", "GenerateTitle").Dur("dur", time.Since(start)).Msg("llm request")
		return "", err
	}
	log.Info().Str("op", "GenerateTitle").Dur("dur", time.Since(start)).Msg("llm request")
	title := strings.TrimSpace(extractText(result))
	if title == "" {
		title = prompt
		title = truncateRunesWithEllipsis(title, 60)
	}
	return title, nil
}

func truncateRunesWithEllipsis(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	if maxRunes <= 3 {
		return strings.Repeat(".", maxRunes)
	}
	return string(runes[:maxRunes-3]) + "..."
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

	streamCfg := &genai.GenerateContentConfig{SafetySettings: defaultSafetySettings()}
	var full strings.Builder
	for resp, err := range c.client.Models.GenerateContentStream(ctx, c.model, []*genai.Content{
		{Parts: []*genai.Part{{Text: prompt}}},
	}, streamCfg) {
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
		SafetySettings:     defaultSafetySettings(),
		SpeechConfig: &genai.SpeechConfig{
			VoiceConfig: &genai.VoiceConfig{
				PrebuiltVoiceConfig: &genai.PrebuiltVoiceConfig{VoiceName: "Puck"},
			},
		},
	}
	contents := []*genai.Content{
		{Role: "user", Parts: []*genai.Part{{Text: "Generate audio from the following script: " + script}}},
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

// timestampsSchema returns a JSON schema for structured output: array of { start_sec }.
func timestampsSchema(numSegments int) *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeArray,
		Items: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"start_sec": {
					Type:        genai.TypeNumber,
					Description: "Start time of this narration segment in seconds (float, 0-indexed). The first segment should start at 0.0.",
				},
			},
			Required: []string{"start_sec"},
		},
		MinItems: ptrInt64(int64(numSegments)),
		MaxItems: ptrInt64(int64(numSegments)),
	}
}

func ptrInt64(v int64) *int64 { return &v }

// TimestampEntry is the LLM's structured response for one segment.
type TimestampEntry struct {
	StartSec float64 `json:"start_sec"`
}

// FindAudioTimestamps sends combined TTS audio + the ordered narration texts to a cheap model
// and asks it to return the start-time of each narration segment. Returns one offset per narration.
func (c *Client) FindAudioTimestamps(ctx context.Context, model string, wav []byte, narrations []string) ([]float64, error) {
	start := time.Now()
	defer func() {
		log.Info().Str("op", "FindAudioTimestamps").Str("model", model).Int("segments", len(narrations)).Dur("dur", time.Since(start)).Msg("llm request")
	}()

	var numbered strings.Builder
	for i, n := range narrations {
		fmt.Fprintf(&numbered, "%d. %s\n", i+1, strings.TrimSpace(n))
	}

	systemPrompt := fmt.Sprintf(`You are an audio timestamp detector. You are given an audio file containing %d narration segments spoken one after another.
Listen to the audio carefully and return the start time (in seconds, as a float) of each segment.
The first segment always starts at 0.0.
Return exactly %d entries.`, len(narrations), len(narrations))

	userPrompt := fmt.Sprintf("Here are the narration texts in the exact order they appear in the audio:\n\n%s", numbered.String())

	cfg := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{{Text: systemPrompt}},
		},
		ResponseMIMEType: "application/json",
		ResponseSchema:   timestampsSchema(len(narrations)),
		SafetySettings:   defaultSafetySettings(),
	}
	result, err := c.client.Models.GenerateContent(ctx, model, []*genai.Content{
		{Role: "user", Parts: []*genai.Part{
			{InlineData: &genai.Blob{Data: wav, MIMEType: "audio/wav"}},
			{Text: userPrompt},
		}},
	}, cfg)
	if err != nil {
		log.Error().Err(err).Str("op", "FindAudioTimestamps").Str("model", model).Dur("dur", time.Since(start)).Msg("llm request")
		return nil, fmt.Errorf("FindAudioTimestamps: %w", err)
	}
	text := extractText(result)
	var entries []TimestampEntry
	if err := json.Unmarshal([]byte(text), &entries); err != nil {
		return nil, fmt.Errorf("parse timestamps JSON: %w\nraw: %.500s", err, text)
	}
	if len(entries) != len(narrations) {
		return nil, fmt.Errorf("expected %d timestamps, got %d", len(narrations), len(entries))
	}
	offsets := make([]float64, len(entries))
	for i, e := range entries {
		offsets[i] = e.StartSec
	}
	return offsets, nil
}

// PCMToWAV wraps raw 16-bit LE mono PCM data in a WAV header.
func PCMToWAV(pcm []byte, sampleRate int) []byte {
	dataLen := len(pcm)
	fileLen := 36 + dataLen
	buf := make([]byte, 44+dataLen)
	copy(buf[0:4], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:8], uint32(fileLen))
	copy(buf[8:12], "WAVE")
	copy(buf[12:16], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:20], 16) // PCM chunk size
	binary.LittleEndian.PutUint16(buf[20:22], 1)  // PCM format
	binary.LittleEndian.PutUint16(buf[22:24], 1)  // mono
	binary.LittleEndian.PutUint32(buf[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(buf[28:32], uint32(sampleRate*2)) // byte rate (16-bit mono)
	binary.LittleEndian.PutUint16(buf[32:34], 2)                    // block align
	binary.LittleEndian.PutUint16(buf[34:36], 16)                   // bits per sample
	copy(buf[36:40], "data")
	binary.LittleEndian.PutUint32(buf[40:44], uint32(dataLen))
	copy(buf[44:], pcm)
	return buf
}

// SplitPCMAtOffsets splits raw 16-bit LE PCM into segments at the given second offsets.
// offsets[0] should be 0.0 (or close to it); offsets must be sorted ascending.
func SplitPCMAtOffsets(pcm []byte, sampleRate int, offsets []float64) [][]byte {
	if len(offsets) == 0 {
		return nil
	}
	totalSamples := len(pcm) / 2
	out := make([][]byte, 0, len(offsets))
	for i, off := range offsets {
		startSample := int(off * float64(sampleRate))
		if startSample < 0 {
			startSample = 0
		}
		if startSample > totalSamples {
			startSample = totalSamples
		}
		var endSample int
		if i+1 < len(offsets) {
			endSample = int(offsets[i+1] * float64(sampleRate))
		} else {
			endSample = totalSamples
		}
		if endSample > totalSamples {
			endSample = totalSamples
		}
		if endSample < startSample {
			endSample = startSample
		}
		out = append(out, pcm[startSample*2:endSample*2])
	}
	return out
}
