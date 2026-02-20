package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ModelsLab/cli/internal/output"
	"github.com/spf13/cobra"
)

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate AI content (image, video, audio, 3D, chat)",
	Aliases: []string{"gen"},
}

// Common generation flags
func addGenerationFlags(cmd *cobra.Command) {
	cmd.Flags().String("model", "", "Model ID to use")
	cmd.Flags().String("webhook", "", "Webhook URL for async notification")
	cmd.Flags().Bool("no-wait", false, "Return immediately without polling")
	cmd.Flags().Bool("download", true, "Auto-download output files")
	cmd.Flags().String("output-dir", "./generated", "Output directory for downloads")
	cmd.Flags().Duration("timeout", 5*time.Minute, "Polling timeout")
}

// pollAndDownload handles the async polling + download flow for generation commands.
func pollAndDownload(cmd *cobra.Command, genType, fetchEndpoint string, result map[string]interface{}) error {
	noWait, _ := cmd.Flags().GetBool("no-wait")
	download, _ := cmd.Flags().GetBool("download")
	outputDir, _ := cmd.Flags().GetString("output-dir")
	timeout, _ := cmd.Flags().GetDuration("timeout")

	// Extract request ID
	requestID := ""
	status := ""
	if id, ok := result["id"].(float64); ok {
		requestID = fmt.Sprintf("%.0f", id)
	} else if id, ok := result["id"].(string); ok {
		requestID = id
	}
	if s, ok := result["status"].(string); ok {
		status = s
	}

	// If output is already available
	if status == "success" || hasOutputURLs(result) {
		return handleCompleted(result, download, outputDir, genType)
	}

	if noWait {
		outputResult(result, func() {
			fmt.Printf("Job queued: %s\n", requestID)
			fmt.Printf("Check status: modelslab generate fetch --id %s --type %s\n", requestID, genType)
		})
		return nil
	}

	// Poll for completion
	client := getClient()
	startTime := time.Now()
	pollInterval := 1 * time.Second
	maxInterval := 10 * time.Second

	for {
		if time.Since(startTime) > timeout {
			fmt.Fprintf(os.Stderr, "Timeout waiting for generation. Job ID: %s\n", requestID)
			fmt.Fprintf(os.Stderr, "Check later: modelslab generate fetch --id %s --type %s\n", requestID, genType)
			os.Exit(7)
		}

		time.Sleep(pollInterval)

		var fetchResult map[string]interface{}
		err := client.DoGeneration("POST", fetchEndpoint+"/"+requestID, map[string]string{
			"key": client.APIKey,
		}, &fetchResult)
		if err != nil {
			// Retry on transient errors
			pollInterval = minDuration(pollInterval*2, maxInterval)
			continue
		}

		fetchStatus := ""
		if s, ok := fetchResult["status"].(string); ok {
			fetchStatus = s
		}

		switch fetchStatus {
		case "success":
			return handleCompleted(fetchResult, download, outputDir, genType)
		case "error", "failed":
			msg := "Generation failed"
			if m, ok := fetchResult["message"].(string); ok {
				msg = m
			}
			return fmt.Errorf("%s", msg)
		case "processing":
			elapsed := time.Since(startTime).Round(time.Second)
			eta := ""
			if e, ok := fetchResult["eta"].(float64); ok && e > 0 {
				eta = fmt.Sprintf(" (ETA: %.0fs)", e)
			}
			fmt.Fprintf(os.Stderr, "\rProcessing... %s%s", elapsed, eta)
		}

		pollInterval = minDuration(pollInterval*2, maxInterval)
	}
}

func hasOutputURLs(result map[string]interface{}) bool {
	if output, ok := result["output"].([]interface{}); ok && len(output) > 0 {
		return true
	}
	if _, ok := result["output_url"].(string); ok {
		return true
	}
	return false
}

func handleCompleted(result map[string]interface{}, download bool, outputDir, genType string) error {
	fmt.Fprintln(os.Stderr) // Clear progress line

	urls := extractOutputURLs(result)

	if download && len(urls) > 0 {
		os.MkdirAll(outputDir, 0755)
		for i, url := range urls {
			ext := guessExtension(genType, url)
			filename := filepath.Join(outputDir, fmt.Sprintf("%s_%d%s", genType, time.Now().UnixMilli(), ext))
			if i > 0 {
				filename = filepath.Join(outputDir, fmt.Sprintf("%s_%d_%d%s", genType, time.Now().UnixMilli(), i, ext))
			}
			if err := downloadFile(url, filename); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not download %s: %v\n", url, err)
			} else {
				output.PrintSuccess(fmt.Sprintf("Saved to %s", filename))
			}
		}
	}

	outputResult(result, func() {
		if len(urls) > 0 {
			for _, url := range urls {
				fmt.Println(url)
			}
		}
	})
	return nil
}

func extractOutputURLs(result map[string]interface{}) []string {
	var urls []string
	if output, ok := result["output"].([]interface{}); ok {
		for _, u := range output {
			if s, ok := u.(string); ok {
				urls = append(urls, s)
			}
		}
	}
	if url, ok := result["output_url"].(string); ok && url != "" {
		urls = append(urls, url)
	}
	if url, ok := result["future_links"].([]interface{}); ok {
		for _, u := range url {
			if s, ok := u.(string); ok {
				urls = append(urls, s)
			}
		}
	}
	return urls
}

func guessExtension(genType, url string) string {
	lower := strings.ToLower(url)
	for _, ext := range []string{".png", ".jpg", ".jpeg", ".webp", ".mp4", ".mp3", ".wav", ".glb", ".obj", ".fbx"} {
		if strings.Contains(lower, ext) {
			return ext
		}
	}
	switch genType {
	case "image":
		return ".png"
	case "video":
		return ".mp4"
	case "audio", "tts", "music", "sfx":
		return ".mp3"
	case "3d":
		return ".glb"
	default:
		return ".bin"
	}
}

func downloadFile(url, filename string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

// --- generate image ---
var generateImageCmd = &cobra.Command{
	Use:   "image",
	Short: "Generate image from text",
	RunE: func(cmd *cobra.Command, args []string) error {
		prompt, _ := cmd.Flags().GetString("prompt")
		if prompt == "" {
			return fmt.Errorf("--prompt is required")
		}

		body := map[string]interface{}{
			"key":    getClient().APIKey,
			"prompt": prompt,
		}
		if v, _ := cmd.Flags().GetString("negative-prompt"); v != "" {
			body["negative_prompt"] = v
		}
		if v, _ := cmd.Flags().GetString("model"); v != "" {
			body["model_id"] = v
		}
		if v, _ := cmd.Flags().GetInt("width"); v > 0 {
			body["width"] = v
		}
		if v, _ := cmd.Flags().GetInt("height"); v > 0 {
			body["height"] = v
		}
		if v, _ := cmd.Flags().GetInt("samples"); v > 0 {
			body["samples"] = v
		}
		if v, _ := cmd.Flags().GetInt("steps"); v > 0 {
			body["num_inference_steps"] = v
		}
		if v, _ := cmd.Flags().GetFloat64("guidance"); v > 0 {
			body["guidance_scale"] = v
		}
		if v, _ := cmd.Flags().GetInt64("seed"); v > 0 {
			body["seed"] = v
		}
		if v, _ := cmd.Flags().GetString("scheduler"); v != "" {
			body["scheduler"] = v
		}
		if v, _ := cmd.Flags().GetString("webhook"); v != "" {
			body["webhook"] = v
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoGeneration("POST", "/v7/images/text-to-image", body, &result)
		if err != nil {
			return err
		}

		return pollAndDownload(cmd, "image", "/v7/images/fetch", result)
	},
}

// --- generate image-to-image ---
var generateImg2ImgCmd = &cobra.Command{
	Use:   "image-to-image",
	Short: "Transform an image",
	RunE: func(cmd *cobra.Command, args []string) error {
		prompt, _ := cmd.Flags().GetString("prompt")
		initImage, _ := cmd.Flags().GetString("init-image")

		body := map[string]interface{}{
			"key":        getClient().APIKey,
			"prompt":     prompt,
			"init_image": initImage,
		}
		if v, _ := cmd.Flags().GetString("model"); v != "" {
			body["model_id"] = v
		}
		if v, _ := cmd.Flags().GetFloat64("strength"); v > 0 {
			body["strength"] = v
		}
		if v, _ := cmd.Flags().GetString("webhook"); v != "" {
			body["webhook"] = v
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoGeneration("POST", "/v7/images/image-to-image", body, &result)
		if err != nil {
			return err
		}

		return pollAndDownload(cmd, "image", "/v7/images/fetch", result)
	},
}

// --- generate inpaint ---
var generateInpaintCmd = &cobra.Command{
	Use:   "inpaint",
	Short: "Inpaint an image",
	RunE: func(cmd *cobra.Command, args []string) error {
		prompt, _ := cmd.Flags().GetString("prompt")
		initImage, _ := cmd.Flags().GetString("init-image")
		maskImage, _ := cmd.Flags().GetString("mask-image")

		body := map[string]interface{}{
			"key":        getClient().APIKey,
			"prompt":     prompt,
			"init_image": initImage,
			"mask_image": maskImage,
		}
		if v, _ := cmd.Flags().GetString("model"); v != "" {
			body["model_id"] = v
		}
		if v, _ := cmd.Flags().GetString("webhook"); v != "" {
			body["webhook"] = v
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoGeneration("POST", "/v7/images/inpaint", body, &result)
		if err != nil {
			return err
		}

		return pollAndDownload(cmd, "image", "/v7/images/fetch", result)
	},
}

// --- generate video ---
var generateVideoCmd = &cobra.Command{
	Use:   "video",
	Short: "Generate video from text",
	RunE: func(cmd *cobra.Command, args []string) error {
		prompt, _ := cmd.Flags().GetString("prompt")
		body := map[string]interface{}{
			"key":    getClient().APIKey,
			"prompt": prompt,
		}
		if v, _ := cmd.Flags().GetString("model"); v != "" {
			body["model_id"] = v
		}
		if v, _ := cmd.Flags().GetString("negative-prompt"); v != "" {
			body["negative_prompt"] = v
		}
		if v, _ := cmd.Flags().GetInt("width"); v > 0 {
			body["width"] = v
		}
		if v, _ := cmd.Flags().GetInt("height"); v > 0 {
			body["height"] = v
		}
		if v, _ := cmd.Flags().GetString("webhook"); v != "" {
			body["webhook"] = v
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoGeneration("POST", "/v7/video-fusion/text-to-video", body, &result)
		if err != nil {
			return err
		}

		return pollAndDownload(cmd, "video", "/v7/video-fusion/fetch", result)
	},
}

// --- generate image-to-video ---
var generateImg2VidCmd = &cobra.Command{
	Use:   "image-to-video",
	Short: "Animate an image into video",
	RunE: func(cmd *cobra.Command, args []string) error {
		initImage, _ := cmd.Flags().GetString("init-image")
		body := map[string]interface{}{
			"key":        getClient().APIKey,
			"init_image": initImage,
		}
		if v, _ := cmd.Flags().GetString("prompt"); v != "" {
			body["prompt"] = v
		}
		if v, _ := cmd.Flags().GetString("model"); v != "" {
			body["model_id"] = v
		}
		if v, _ := cmd.Flags().GetString("webhook"); v != "" {
			body["webhook"] = v
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoGeneration("POST", "/v7/video-fusion/image-to-video", body, &result)
		if err != nil {
			return err
		}

		return pollAndDownload(cmd, "video", "/v7/video-fusion/fetch", result)
	},
}

// --- generate video-to-video ---
var generateVid2VidCmd = &cobra.Command{
	Use:   "video-to-video",
	Short: "Transform a video",
	RunE: func(cmd *cobra.Command, args []string) error {
		initVideo, _ := cmd.Flags().GetString("init-video")
		body := map[string]interface{}{
			"key":        getClient().APIKey,
			"init_video": initVideo,
		}
		if v, _ := cmd.Flags().GetString("prompt"); v != "" {
			body["prompt"] = v
		}
		if v, _ := cmd.Flags().GetString("model"); v != "" {
			body["model_id"] = v
		}
		if v, _ := cmd.Flags().GetString("webhook"); v != "" {
			body["webhook"] = v
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoGeneration("POST", "/v7/video-fusion/video-to-video", body, &result)
		if err != nil {
			return err
		}

		return pollAndDownload(cmd, "video", "/v7/video-fusion/fetch", result)
	},
}

// --- generate lip-sync ---
var generateLipSyncCmd = &cobra.Command{
	Use:   "lip-sync",
	Short: "Lip synchronization",
	RunE: func(cmd *cobra.Command, args []string) error {
		body := map[string]interface{}{
			"key": getClient().APIKey,
		}
		if v, _ := cmd.Flags().GetString("init-video"); v != "" {
			body["init_video"] = v
		}
		if v, _ := cmd.Flags().GetString("init-audio"); v != "" {
			body["init_audio"] = v
		}
		if v, _ := cmd.Flags().GetString("model"); v != "" {
			body["model_id"] = v
		}
		if v, _ := cmd.Flags().GetString("webhook"); v != "" {
			body["webhook"] = v
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoGeneration("POST", "/v7/video-fusion/lip-sync", body, &result)
		if err != nil {
			return err
		}

		return pollAndDownload(cmd, "video", "/v7/video-fusion/fetch", result)
	},
}

// --- generate motion-control ---
var generateMotionControlCmd = &cobra.Command{
	Use:   "motion-control",
	Short: "Motion control generation",
	RunE: func(cmd *cobra.Command, args []string) error {
		body := map[string]interface{}{
			"key": getClient().APIKey,
		}
		if v, _ := cmd.Flags().GetString("init-image"); v != "" {
			body["init_image"] = v
		}
		if v, _ := cmd.Flags().GetString("prompt"); v != "" {
			body["prompt"] = v
		}
		if v, _ := cmd.Flags().GetString("model"); v != "" {
			body["model_id"] = v
		}
		if v, _ := cmd.Flags().GetString("webhook"); v != "" {
			body["webhook"] = v
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoGeneration("POST", "/v7/video-fusion/motion-control", body, &result)
		if err != nil {
			return err
		}

		return pollAndDownload(cmd, "video", "/v7/video-fusion/fetch", result)
	},
}

// --- generate tts ---
var generateTTSCmd = &cobra.Command{
	Use:   "tts",
	Short: "Text-to-speech",
	RunE: func(cmd *cobra.Command, args []string) error {
		text, _ := cmd.Flags().GetString("text")
		body := map[string]interface{}{
			"key":  getClient().APIKey,
			"text": text,
		}
		if v, _ := cmd.Flags().GetString("model"); v != "" {
			body["model_id"] = v
		}
		if v, _ := cmd.Flags().GetString("language"); v != "" {
			body["language"] = v
		}
		if v, _ := cmd.Flags().GetString("webhook"); v != "" {
			body["webhook"] = v
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoGeneration("POST", "/v7/voice/text-to-speech", body, &result)
		if err != nil {
			return err
		}

		return pollAndDownload(cmd, "audio", "/v7/voice/fetch", result)
	},
}

// --- generate stt ---
var generateSTTCmd = &cobra.Command{
	Use:   "stt",
	Short: "Speech-to-text",
	RunE: func(cmd *cobra.Command, args []string) error {
		initAudio, _ := cmd.Flags().GetString("init-audio")
		body := map[string]interface{}{
			"key":        getClient().APIKey,
			"init_audio": initAudio,
		}
		if v, _ := cmd.Flags().GetString("model"); v != "" {
			body["model_id"] = v
		}
		if v, _ := cmd.Flags().GetString("webhook"); v != "" {
			body["webhook"] = v
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoGeneration("POST", "/v7/voice/speech-to-text", body, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			if text, ok := result["output"].(string); ok {
				fmt.Println(text)
			} else {
				output.PrintJSON(result)
			}
		})
		return nil
	},
}

// --- generate voice-to-voice ---
var generateV2VCmd = &cobra.Command{
	Use:   "voice-to-voice",
	Short: "Voice conversion",
	RunE: func(cmd *cobra.Command, args []string) error {
		initAudio, _ := cmd.Flags().GetString("init-audio")
		body := map[string]interface{}{
			"key":        getClient().APIKey,
			"init_audio": initAudio,
		}
		if v, _ := cmd.Flags().GetString("model"); v != "" {
			body["model_id"] = v
		}
		if v, _ := cmd.Flags().GetString("webhook"); v != "" {
			body["webhook"] = v
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoGeneration("POST", "/v7/voice/speech-to-speech", body, &result)
		if err != nil {
			return err
		}

		return pollAndDownload(cmd, "audio", "/v7/voice/fetch", result)
	},
}

// --- generate music ---
var generateMusicCmd = &cobra.Command{
	Use:   "music",
	Short: "Generate music",
	RunE: func(cmd *cobra.Command, args []string) error {
		prompt, _ := cmd.Flags().GetString("prompt")
		body := map[string]interface{}{
			"key":    getClient().APIKey,
			"prompt": prompt,
		}
		if v, _ := cmd.Flags().GetString("model"); v != "" {
			body["model_id"] = v
		}
		if v, _ := cmd.Flags().GetFloat64("duration"); v > 0 {
			body["duration"] = v
		}
		if v, _ := cmd.Flags().GetString("webhook"); v != "" {
			body["webhook"] = v
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoGeneration("POST", "/v7/voice/music-gen", body, &result)
		if err != nil {
			return err
		}

		return pollAndDownload(cmd, "audio", "/v7/voice/fetch", result)
	},
}

// --- generate sfx ---
var generateSFXCmd = &cobra.Command{
	Use:   "sfx",
	Short: "Generate sound effects",
	RunE: func(cmd *cobra.Command, args []string) error {
		prompt, _ := cmd.Flags().GetString("prompt")
		body := map[string]interface{}{
			"key":    getClient().APIKey,
			"prompt": prompt,
		}
		if v, _ := cmd.Flags().GetString("model"); v != "" {
			body["model_id"] = v
		}
		if v, _ := cmd.Flags().GetString("webhook"); v != "" {
			body["webhook"] = v
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoGeneration("POST", "/v7/voice/sound-generation", body, &result)
		if err != nil {
			return err
		}

		return pollAndDownload(cmd, "audio", "/v7/voice/fetch", result)
	},
}

// --- generate dubbing ---
var generateDubbingCmd = &cobra.Command{
	Use:   "dubbing",
	Short: "Video dubbing",
	RunE: func(cmd *cobra.Command, args []string) error {
		body := map[string]interface{}{
			"key": getClient().APIKey,
		}
		if v, _ := cmd.Flags().GetString("init-video"); v != "" {
			body["init_video"] = v
		}
		if v, _ := cmd.Flags().GetString("target-language"); v != "" {
			body["target_language"] = v
		}
		if v, _ := cmd.Flags().GetString("webhook"); v != "" {
			body["webhook"] = v
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoGeneration("POST", "/v7/voice/create-dubbing", body, &result)
		if err != nil {
			return err
		}

		return pollAndDownload(cmd, "audio", "/v7/voice/fetch", result)
	},
}

// --- generate song-extend ---
var generateSongExtendCmd = &cobra.Command{
	Use:   "song-extend",
	Short: "Extend a song",
	RunE: func(cmd *cobra.Command, args []string) error {
		body := map[string]interface{}{
			"key": getClient().APIKey,
		}
		if v, _ := cmd.Flags().GetString("init-audio"); v != "" {
			body["init_audio"] = v
		}
		if v, _ := cmd.Flags().GetString("prompt"); v != "" {
			body["prompt"] = v
		}
		if v, _ := cmd.Flags().GetString("webhook"); v != "" {
			body["webhook"] = v
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoGeneration("POST", "/v7/voice/song-extender", body, &result)
		if err != nil {
			return err
		}

		return pollAndDownload(cmd, "audio", "/v7/voice/fetch", result)
	},
}

// --- generate song-inpaint ---
var generateSongInpaintCmd = &cobra.Command{
	Use:   "song-inpaint",
	Short: "Edit song sections",
	RunE: func(cmd *cobra.Command, args []string) error {
		body := map[string]interface{}{
			"key": getClient().APIKey,
		}
		if v, _ := cmd.Flags().GetString("init-audio"); v != "" {
			body["init_audio"] = v
		}
		if v, _ := cmd.Flags().GetString("prompt"); v != "" {
			body["prompt"] = v
		}
		if v, _ := cmd.Flags().GetString("webhook"); v != "" {
			body["webhook"] = v
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoGeneration("POST", "/v7/voice/song-inpaint", body, &result)
		if err != nil {
			return err
		}

		return pollAndDownload(cmd, "audio", "/v7/voice/fetch", result)
	},
}

// --- generate text-to-3d ---
var generateText3DCmd = &cobra.Command{
	Use:   "text-to-3d",
	Short: "Generate 3D model from text",
	RunE: func(cmd *cobra.Command, args []string) error {
		prompt, _ := cmd.Flags().GetString("prompt")
		body := map[string]interface{}{
			"key":    getClient().APIKey,
			"prompt": prompt,
		}
		if v, _ := cmd.Flags().GetString("webhook"); v != "" {
			body["webhook"] = v
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoGeneration("POST", "/v6/3d/text_to_3d", body, &result)
		if err != nil {
			return err
		}

		return pollAndDownload(cmd, "3d", "/v6/3d/fetch", result)
	},
}

// --- generate image-to-3d ---
var generateImg3DCmd = &cobra.Command{
	Use:   "image-to-3d",
	Short: "Generate 3D model from image",
	RunE: func(cmd *cobra.Command, args []string) error {
		initImage, _ := cmd.Flags().GetString("init-image")
		body := map[string]interface{}{
			"key":        getClient().APIKey,
			"init_image": initImage,
		}
		if v, _ := cmd.Flags().GetString("webhook"); v != "" {
			body["webhook"] = v
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoGeneration("POST", "/v6/3d/image_to_3d", body, &result)
		if err != nil {
			return err
		}

		return pollAndDownload(cmd, "3d", "/v6/3d/fetch", result)
	},
}

// --- generate chat ---
var generateChatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Chat completion (OpenAI-compatible)",
	RunE: func(cmd *cobra.Command, args []string) error {
		message, _ := cmd.Flags().GetString("message")
		model, _ := cmd.Flags().GetString("model")
		systemPrompt, _ := cmd.Flags().GetString("system")

		messages := []map[string]string{}
		if systemPrompt != "" {
			messages = append(messages, map[string]string{"role": "system", "content": systemPrompt})
		}
		messages = append(messages, map[string]string{"role": "user", "content": message})

		body := map[string]interface{}{
			"key":      getClient().APIKey,
			"messages": messages,
		}
		if model != "" {
			body["model"] = model
		}
		if v, _ := cmd.Flags().GetFloat64("temperature"); v > 0 {
			body["temperature"] = v
		}
		if v, _ := cmd.Flags().GetInt("max-tokens"); v > 0 {
			body["max_tokens"] = v
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoGeneration("POST", "/v7/llm/chat/completions", body, &result)
		if err != nil {
			return err
		}

		outputResult(result, func() {
			// Try to extract the message content
			if choices, ok := result["choices"].([]interface{}); ok && len(choices) > 0 {
				if choice, ok := choices[0].(map[string]interface{}); ok {
					if msg, ok := choice["message"].(map[string]interface{}); ok {
						if content, ok := msg["content"].(string); ok {
							fmt.Println(content)
							return
						}
					}
				}
			}
			// Try simple output field
			if msg, ok := result["message"].(string); ok {
				fmt.Println(msg)
				return
			}
			output.PrintJSON(result)
		})
		return nil
	},
}

// --- generate fetch ---
var generateFetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "Check generation status",
	RunE: func(cmd *cobra.Command, args []string) error {
		id, _ := cmd.Flags().GetString("id")
		genType, _ := cmd.Flags().GetString("type")

		if id == "" {
			return fmt.Errorf("--id is required")
		}

		endpoints := map[string]string{
			"image": "/v7/images/fetch",
			"video": "/v7/video-fusion/fetch",
			"audio": "/v7/voice/fetch",
			"3d":    "/v6/3d/fetch",
		}

		endpoint, ok := endpoints[genType]
		if !ok {
			return fmt.Errorf("unknown type %q, must be one of: image, video, audio, 3d", genType)
		}

		client := getClient()
		var result map[string]interface{}
		err := client.DoGeneration("POST", endpoint+"/"+id, map[string]string{
			"key": client.APIKey,
		}, &result)
		if err != nil {
			return err
		}

		download, _ := cmd.Flags().GetBool("download")
		outputDir, _ := cmd.Flags().GetString("output-dir")

		status := ""
		if s, ok := result["status"].(string); ok {
			status = s
		}

		if status == "success" && download {
			return handleCompleted(result, download, outputDir, genType)
		}

		outputResult(result, func() {
			fmt.Printf("Status: %s\n", status)
			if urls := extractOutputURLs(result); len(urls) > 0 {
				for _, url := range urls {
					fmt.Println(url)
				}
			}
		})
		return nil
	},
}

func init() {
	// Image flags
	generateImageCmd.Flags().String("prompt", "", "Text prompt (required)")
	generateImageCmd.Flags().String("negative-prompt", "", "Negative prompt")
	generateImageCmd.Flags().Int("width", 1024, "Image width")
	generateImageCmd.Flags().Int("height", 1024, "Image height")
	generateImageCmd.Flags().Int("samples", 1, "Number of images")
	generateImageCmd.Flags().Int("steps", 0, "Inference steps")
	generateImageCmd.Flags().Float64("guidance", 0, "Guidance scale")
	generateImageCmd.Flags().Int64("seed", 0, "Random seed")
	generateImageCmd.Flags().String("scheduler", "", "Scheduler")
	addGenerationFlags(generateImageCmd)

	// Img2Img flags
	generateImg2ImgCmd.Flags().String("prompt", "", "Text prompt")
	generateImg2ImgCmd.Flags().String("init-image", "", "Source image URL")
	generateImg2ImgCmd.Flags().Float64("strength", 0.7, "Transformation strength")
	addGenerationFlags(generateImg2ImgCmd)

	// Inpaint flags
	generateInpaintCmd.Flags().String("prompt", "", "Text prompt")
	generateInpaintCmd.Flags().String("init-image", "", "Source image URL")
	generateInpaintCmd.Flags().String("mask-image", "", "Mask image URL")
	addGenerationFlags(generateInpaintCmd)

	// Video flags
	generateVideoCmd.Flags().String("prompt", "", "Text prompt")
	generateVideoCmd.Flags().String("negative-prompt", "", "Negative prompt")
	generateVideoCmd.Flags().Int("width", 0, "Video width")
	generateVideoCmd.Flags().Int("height", 0, "Video height")
	addGenerationFlags(generateVideoCmd)

	// Img2Vid flags
	generateImg2VidCmd.Flags().String("init-image", "", "Source image URL")
	generateImg2VidCmd.Flags().String("prompt", "", "Text prompt")
	addGenerationFlags(generateImg2VidCmd)

	// Vid2Vid flags
	generateVid2VidCmd.Flags().String("init-video", "", "Source video URL")
	generateVid2VidCmd.Flags().String("prompt", "", "Text prompt")
	addGenerationFlags(generateVid2VidCmd)

	// Lip-sync flags
	generateLipSyncCmd.Flags().String("init-video", "", "Source video URL")
	generateLipSyncCmd.Flags().String("init-audio", "", "Audio URL")
	addGenerationFlags(generateLipSyncCmd)

	// Motion-control flags
	generateMotionControlCmd.Flags().String("init-image", "", "Source image URL")
	generateMotionControlCmd.Flags().String("prompt", "", "Text prompt")
	addGenerationFlags(generateMotionControlCmd)

	// TTS flags
	generateTTSCmd.Flags().String("text", "", "Text to speak")
	generateTTSCmd.Flags().String("language", "", "Language code")
	addGenerationFlags(generateTTSCmd)

	// STT flags
	generateSTTCmd.Flags().String("init-audio", "", "Audio URL")
	addGenerationFlags(generateSTTCmd)

	// Voice-to-voice flags
	generateV2VCmd.Flags().String("init-audio", "", "Source audio URL")
	addGenerationFlags(generateV2VCmd)

	// Music flags
	generateMusicCmd.Flags().String("prompt", "", "Music description")
	generateMusicCmd.Flags().Float64("duration", 0, "Duration in seconds")
	addGenerationFlags(generateMusicCmd)

	// SFX flags
	generateSFXCmd.Flags().String("prompt", "", "Sound description")
	addGenerationFlags(generateSFXCmd)

	// Dubbing flags
	generateDubbingCmd.Flags().String("init-video", "", "Video URL")
	generateDubbingCmd.Flags().String("target-language", "", "Target language")
	addGenerationFlags(generateDubbingCmd)

	// Song extend flags
	generateSongExtendCmd.Flags().String("init-audio", "", "Audio URL")
	generateSongExtendCmd.Flags().String("prompt", "", "Extension prompt")
	addGenerationFlags(generateSongExtendCmd)

	// Song inpaint flags
	generateSongInpaintCmd.Flags().String("init-audio", "", "Audio URL")
	generateSongInpaintCmd.Flags().String("prompt", "", "Edit prompt")
	addGenerationFlags(generateSongInpaintCmd)

	// 3D flags
	generateText3DCmd.Flags().String("prompt", "", "3D description")
	addGenerationFlags(generateText3DCmd)

	generateImg3DCmd.Flags().String("init-image", "", "Source image URL")
	addGenerationFlags(generateImg3DCmd)

	// Chat flags
	generateChatCmd.Flags().String("message", "", "Chat message")
	generateChatCmd.Flags().String("system", "", "System prompt")
	generateChatCmd.Flags().Float64("temperature", 0, "Temperature")
	generateChatCmd.Flags().Int("max-tokens", 0, "Max tokens")
	addGenerationFlags(generateChatCmd)

	// Fetch flags
	generateFetchCmd.Flags().String("id", "", "Job ID")
	generateFetchCmd.Flags().String("type", "image", "Job type: image, video, audio, 3d")
	generateFetchCmd.Flags().Bool("download", true, "Download output")
	generateFetchCmd.Flags().String("output-dir", "./generated", "Output directory")

	// Suppress unused imports
	_ = json.Marshal
	_ = strings.Join

	generateCmd.AddCommand(generateImageCmd)
	generateCmd.AddCommand(generateImg2ImgCmd)
	generateCmd.AddCommand(generateInpaintCmd)
	generateCmd.AddCommand(generateVideoCmd)
	generateCmd.AddCommand(generateImg2VidCmd)
	generateCmd.AddCommand(generateVid2VidCmd)
	generateCmd.AddCommand(generateLipSyncCmd)
	generateCmd.AddCommand(generateMotionControlCmd)
	generateCmd.AddCommand(generateTTSCmd)
	generateCmd.AddCommand(generateSTTCmd)
	generateCmd.AddCommand(generateV2VCmd)
	generateCmd.AddCommand(generateMusicCmd)
	generateCmd.AddCommand(generateSFXCmd)
	generateCmd.AddCommand(generateDubbingCmd)
	generateCmd.AddCommand(generateSongExtendCmd)
	generateCmd.AddCommand(generateSongInpaintCmd)
	generateCmd.AddCommand(generateText3DCmd)
	generateCmd.AddCommand(generateImg3DCmd)
	generateCmd.AddCommand(generateChatCmd)
	generateCmd.AddCommand(generateFetchCmd)
}
