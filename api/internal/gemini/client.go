package gemini

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

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

// GenerateCSS produces a single block of CSS for the code view/theme/layout.
func (c *Client) GenerateCSS(ctx context.Context, spec string) (css string, err error) {
	prompt := fmt.Sprintf(`Generate a single block of CSS for a code viewer page. The page shows code in a monospace editor with a nice theme. Context: %s

Output only valid CSS, no markdown code fences. Include:
- A container for the code view (e.g. .code-view or #code-view)
- Syntax-friendly colors (background, text, maybe keyword/string/comment colors)
- Readable font and padding`, spec)

	result, err := c.client.Models.GenerateContent(ctx, c.model, []*genai.Content{
		{Parts: []*genai.Part{{Text: prompt}}},
	}, nil)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(cleanCodeBlock(extractText(result))), nil
}

// GenerateCode produces full source code in the requested language.
func (c *Client) GenerateCode(ctx context.Context, spec, language string) (code string, err error) {
	prompt := fmt.Sprintf(`Generate full source code that fulfills this spec. Language: %s

Spec: %s

Output only the code, no markdown code fences or explanation.`, language, spec)

	result, err := c.client.Models.GenerateContent(ctx, c.model, []*genai.Content{
		{Parts: []*genai.Part{{Text: prompt}}},
	}, nil)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(cleanCodeBlock(extractText(result))), nil
}

// CodeSegment is one logical part of the code with its narration (for synced voiceover).
type CodeSegment struct {
	Code      string
	Narration string
}

// GenerateCodeSegments returns code split into logical segments, each with a narration line that describes that part (for synced narration).
func (c *Client) GenerateCodeSegments(ctx context.Context, spec, language string) ([]CodeSegment, error) {
	prompt := fmt.Sprintf(`Generate source code that fulfills this spec. Language: %s

Spec: %s

Split the code into 3 to 8 logical segments (e.g. imports, then each function or logical block). Preserve indentation: every line in each segment must keep its exact leading spaces/tabs; continuation segments inside a block must start with the same indentation as the line they continue.

For each segment output exactly this format, no other text:

---SEGMENT---
<code for this segment only, with indentation preserved>
---NARRATION---
<one or two short sentences describing what this segment does, for a voiceover>
---END---

Repeat ---SEGMENT--- / ---NARRATION--- / ---END--- for each segment. Output only valid code in segments and clear narration. No markdown fences.`, language, spec)

	result, err := c.client.Models.GenerateContent(ctx, c.model, []*genai.Content{
		{Parts: []*genai.Part{{Text: prompt}}},
	}, nil)
	if err != nil {
		return nil, err
	}
	text := extractText(result)
	segments := parseSegments(text)
	return segments, nil
}

func parseSegments(text string) []CodeSegment {
	var out []CodeSegment
	text = strings.TrimSpace(text)
	for {
		segStart := strings.Index(text, "---SEGMENT---")
		if segStart < 0 {
			break
		}
		text = text[segStart+13:]
		narStart := strings.Index(text, "---NARRATION---")
		if narStart < 0 {
			break
		}
		code := strings.TrimRight(text[:narStart], " \t\n\r")
		text = text[narStart+15:]
		endIdx := strings.Index(text, "---END---")
		if endIdx < 0 {
			break
		}
		narration := strings.TrimSpace(text[:endIdx])
		text = strings.TrimSpace(text[endIdx+9:])
		if code != "" || narration != "" {
			out = append(out, CodeSegment{Code: code, Narration: narration})
		}
	}
	return out
}

// GenerateCodeStream yields code text chunks via the given callback. Each chunk is a delta; full code can be built by concatenation.
func (c *Client) GenerateCodeStream(ctx context.Context, spec, language string, yield func(chunk string) error) (fullCode string, err error) {
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
func (c *Client) GenerateAudioStream(ctx context.Context, ttsModel, script string, yield func(base64Chunk string) error) error {
	if ttsModel == "" {
		ttsModel = "gemini-2.5-pro-preview-tts"
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

	result, err := c.client.Models.GenerateContent(ctx, c.model, []*genai.Content{
		{Parts: []*genai.Part{{Text: prompt}}},
	}, nil)
	if err != nil {
		return "", "", "", err
	}
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
