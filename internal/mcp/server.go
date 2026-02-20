package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ModelsLab/cli/internal/api"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type ToolInfo struct {
	Name        string
	Description string
}

type Server struct {
	client    *api.Client
	mcpServer *server.MCPServer
}

func NewServer(client *api.Client) *Server {
	s := &Server{client: client}
	s.init()
	return s
}

func (s *Server) init() {
	s.mcpServer = server.NewMCPServer(
		"ModelsLab",
		"1.0.0",
		server.WithResourceCapabilities(true, true),
	)

	// Register all tools
	s.registerControlPlaneTools()
	s.registerGenerationTools()
}

func (s *Server) registerControlPlaneTools() {
	// Auth tools
	s.addTool("auth-login", "Login to ModelsLab and get a bearer token", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"email":       map[string]string{"type": "string", "description": "Account email"},
			"password":    map[string]string{"type": "string", "description": "Account password"},
			"expiry":      map[string]string{"type": "string", "description": "Token expiry (1_week, 1_month, etc.)"},
			"device_name": map[string]string{"type": "string", "description": "Device name"},
		},
		"required": []string{"email", "password"},
	}, func(args map[string]interface{}) (interface{}, error) {
		var result map[string]interface{}
		err := s.client.DoControlPlane("POST", "/auth/login", args, &result)
		return result, err
	})

	s.addTool("auth-signup", "Create a new ModelsLab account", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"email":    map[string]string{"type": "string", "description": "Account email"},
			"password": map[string]string{"type": "string", "description": "Password"},
			"name":     map[string]string{"type": "string", "description": "Display name"},
		},
		"required": []string{"email", "password", "name"},
	}, func(args map[string]interface{}) (interface{}, error) {
		args["password_confirmation"] = args["password"]
		var result map[string]interface{}
		err := s.client.DoControlPlane("POST", "/auth/signup", args, &result)
		return result, err
	})

	// Profile tools
	s.addTool("profile-get", "Get the current user profile", nil, func(args map[string]interface{}) (interface{}, error) {
		var result map[string]interface{}
		err := s.client.DoControlPlane("GET", "/me", nil, &result)
		return result, err
	})

	s.addTool("profile-update", "Update profile fields", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name":     map[string]string{"type": "string", "description": "Display name"},
			"username": map[string]string{"type": "string", "description": "Username"},
			"about":    map[string]string{"type": "string", "description": "About text"},
		},
	}, func(args map[string]interface{}) (interface{}, error) {
		var result map[string]interface{}
		err := s.client.DoControlPlane("PATCH", "/me", args, &result)
		return result, err
	})

	// API Keys tools
	s.addTool("api-keys-list", "List all API keys", nil, func(args map[string]interface{}) (interface{}, error) {
		var result map[string]interface{}
		err := s.client.DoControlPlane("GET", "/api-keys", nil, &result)
		return result, err
	})

	s.addTool("api-keys-create", "Create a new API key", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name":  map[string]string{"type": "string", "description": "Key name"},
			"notes": map[string]string{"type": "string", "description": "Key notes"},
		},
	}, func(args map[string]interface{}) (interface{}, error) {
		var result map[string]interface{}
		err := s.client.DoControlPlane("POST", "/api-keys", args, &result)
		return result, err
	})

	// Models tools
	s.addTool("models-search", "Search 50,000+ AI models", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"search":   map[string]string{"type": "string", "description": "Search query"},
			"feature":  map[string]string{"type": "string", "description": "Feature filter (imagen, video, audio, etc.)"},
			"provider": map[string]string{"type": "string", "description": "Provider filter"},
			"per_page": map[string]interface{}{"type": "integer", "description": "Results per page", "default": 20},
		},
	}, func(args map[string]interface{}) (interface{}, error) {
		path := "/models?"
		for k, v := range args {
			path += fmt.Sprintf("%s=%v&", k, v)
		}
		var result map[string]interface{}
		err := s.client.DoControlPlane("GET", path, nil, &result)
		return result, err
	})

	s.addTool("models-detail", "Get details of a specific model", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"model_id": map[string]string{"type": "string", "description": "Model ID"},
		},
		"required": []string{"model_id"},
	}, func(args map[string]interface{}) (interface{}, error) {
		var result map[string]interface{}
		err := s.client.DoControlPlane("GET", "/models/"+fmt.Sprintf("%v", args["model_id"]), nil, &result)
		return result, err
	})

	// Billing tools
	s.addTool("billing-overview", "View billing overview", nil, func(args map[string]interface{}) (interface{}, error) {
		var result map[string]interface{}
		err := s.client.DoControlPlane("GET", "/billing/overview", nil, &result)
		return result, err
	})

	s.addTool("wallet-balance", "Check wallet balance", nil, func(args map[string]interface{}) (interface{}, error) {
		var result map[string]interface{}
		err := s.client.DoControlPlane("GET", "/wallet/balance", nil, &result)
		return result, err
	})

	s.addTool("wallet-fund", "Add funds to wallet", map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"amount":            map[string]interface{}{"type": "number", "description": "Amount in USD (min $10)"},
			"payment_method_id": map[string]string{"type": "string", "description": "Payment method ID"},
		},
		"required": []string{"amount"},
	}, func(args map[string]interface{}) (interface{}, error) {
		var result map[string]interface{}
		err := s.client.DoControlPlaneIdempotent("POST", "/wallet/fund", args, &result, "")
		return result, err
	})

	// Usage tools
	s.addTool("usage-summary", "Get usage overview", nil, func(args map[string]interface{}) (interface{}, error) {
		var result map[string]interface{}
		err := s.client.DoControlPlane("GET", "/usage/summary", nil, &result)
		return result, err
	})

	// Teams tools
	s.addTool("teams-list", "List team members", nil, func(args map[string]interface{}) (interface{}, error) {
		var result map[string]interface{}
		err := s.client.DoControlPlane("GET", "/teams", nil, &result)
		return result, err
	})

	// Subscriptions tools
	s.addTool("subscriptions-plans", "List available subscription plans", nil, func(args map[string]interface{}) (interface{}, error) {
		var result map[string]interface{}
		err := s.client.DoControlPlane("GET", "/subscriptions/plans", nil, &result)
		return result, err
	})

	s.addTool("subscriptions-list", "List user subscriptions", nil, func(args map[string]interface{}) (interface{}, error) {
		var result map[string]interface{}
		err := s.client.DoControlPlane("GET", "/subscriptions", nil, &result)
		return result, err
	})
}

func (s *Server) registerGenerationTools() {
	genTools := []struct {
		name, desc, endpoint string
		fetchEndpoint        string
		properties           map[string]interface{}
		required             []string
	}{
		{
			"text-to-image", "Generate images from text prompts", "/v7/images/text-to-image", "/v7/images/fetch",
			map[string]interface{}{
				"prompt":          map[string]string{"type": "string", "description": "Text prompt for image generation"},
				"negative_prompt": map[string]string{"type": "string", "description": "Negative prompt"},
				"model_id":        map[string]string{"type": "string", "description": "Model ID"},
				"width":           map[string]interface{}{"type": "integer", "description": "Image width", "default": 1024},
				"height":          map[string]interface{}{"type": "integer", "description": "Image height", "default": 1024},
				"samples":         map[string]interface{}{"type": "integer", "description": "Number of images", "default": 1},
			},
			[]string{"prompt"},
		},
		{
			"image-to-image", "Transform an existing image", "/v7/images/image-to-image", "/v7/images/fetch",
			map[string]interface{}{
				"prompt":     map[string]string{"type": "string", "description": "Text prompt"},
				"init_image": map[string]string{"type": "string", "description": "Source image URL"},
				"model_id":   map[string]string{"type": "string", "description": "Model ID"},
				"strength":   map[string]interface{}{"type": "number", "description": "Transformation strength"},
			},
			[]string{"init_image"},
		},
		{
			"inpaint", "Inpaint an image", "/v7/images/inpaint", "/v7/images/fetch",
			map[string]interface{}{
				"prompt":     map[string]string{"type": "string", "description": "Text prompt"},
				"init_image": map[string]string{"type": "string", "description": "Source image URL"},
				"mask_image": map[string]string{"type": "string", "description": "Mask image URL"},
				"model_id":   map[string]string{"type": "string", "description": "Model ID"},
			},
			[]string{"init_image", "mask_image"},
		},
		{
			"text-to-video", "Generate video from text", "/v7/video-fusion/text-to-video", "/v7/video-fusion/fetch",
			map[string]interface{}{
				"prompt":   map[string]string{"type": "string", "description": "Text prompt"},
				"model_id": map[string]string{"type": "string", "description": "Model ID"},
			},
			[]string{"prompt"},
		},
		{
			"image-to-video", "Animate an image into video", "/v7/video-fusion/image-to-video", "/v7/video-fusion/fetch",
			map[string]interface{}{
				"init_image": map[string]string{"type": "string", "description": "Source image URL"},
				"prompt":     map[string]string{"type": "string", "description": "Text prompt"},
				"model_id":   map[string]string{"type": "string", "description": "Model ID"},
			},
			[]string{"init_image"},
		},
		{
			"text-to-speech", "Convert text to speech", "/v7/voice/text-to-speech", "/v7/voice/fetch",
			map[string]interface{}{
				"text":     map[string]string{"type": "string", "description": "Text to convert"},
				"model_id": map[string]string{"type": "string", "description": "Model ID"},
				"language": map[string]string{"type": "string", "description": "Language code"},
			},
			[]string{"text"},
		},
		{
			"speech-to-text", "Convert speech to text", "/v7/voice/speech-to-text", "/v7/voice/fetch",
			map[string]interface{}{
				"init_audio": map[string]string{"type": "string", "description": "Audio URL"},
				"model_id":   map[string]string{"type": "string", "description": "Model ID"},
			},
			[]string{"init_audio"},
		},
		{
			"music-generation", "Generate music", "/v7/voice/music-gen", "/v7/voice/fetch",
			map[string]interface{}{
				"prompt":   map[string]string{"type": "string", "description": "Music description"},
				"model_id": map[string]string{"type": "string", "description": "Model ID"},
				"duration": map[string]interface{}{"type": "number", "description": "Duration in seconds"},
			},
			[]string{"prompt"},
		},
		{
			"sound-effects", "Generate sound effects", "/v7/voice/sound-generation", "/v7/voice/fetch",
			map[string]interface{}{
				"prompt":   map[string]string{"type": "string", "description": "Sound description"},
				"model_id": map[string]string{"type": "string", "description": "Model ID"},
			},
			[]string{"prompt"},
		},
		{
			"text-to-3d", "Generate 3D model from text", "/v6/3d/text_to_3d", "/v6/3d/fetch",
			map[string]interface{}{
				"prompt": map[string]string{"type": "string", "description": "3D model description"},
			},
			[]string{"prompt"},
		},
		{
			"chat-completion", "Chat completion (OpenAI-compatible)", "/v7/llm/chat/completions", "",
			map[string]interface{}{
				"messages":    map[string]interface{}{"type": "array", "description": "Chat messages array"},
				"model":       map[string]string{"type": "string", "description": "Model name"},
				"temperature": map[string]interface{}{"type": "number", "description": "Temperature"},
				"max_tokens":  map[string]interface{}{"type": "integer", "description": "Max tokens"},
			},
			[]string{"messages"},
		},
	}

	for _, t := range genTools {
		tool := t
		schema := map[string]interface{}{
			"type":       "object",
			"properties": tool.properties,
		}
		if len(tool.required) > 0 {
			schema["required"] = tool.required
		}

		s.addTool(tool.name, tool.desc, schema, func(args map[string]interface{}) (interface{}, error) {
			args["key"] = s.client.APIKey
			var result map[string]interface{}
			err := s.client.DoGeneration("POST", tool.endpoint, args, &result)
			return result, err
		})
	}

	// Fetch tools
	for _, fetchType := range []struct{ name, endpoint string }{
		{"fetch-image", "/v7/images/fetch"},
		{"fetch-video", "/v7/video-fusion/fetch"},
		{"fetch-audio", "/v7/voice/fetch"},
		{"fetch-3d", "/v6/3d/fetch"},
	} {
		ft := fetchType
		s.addTool(ft.name, "Check generation status for "+ft.name[6:], map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id": map[string]string{"type": "string", "description": "Job ID"},
			},
			"required": []string{"id"},
		}, func(args map[string]interface{}) (interface{}, error) {
			id := fmt.Sprintf("%v", args["id"])
			var result map[string]interface{}
			err := s.client.DoGeneration("POST", ft.endpoint+"/"+id, map[string]string{
				"key": s.client.APIKey,
			}, &result)
			return result, err
		})
	}
}

type toolHandler func(args map[string]interface{}) (interface{}, error)

func (s *Server) addTool(name, description string, schema map[string]interface{}, handler toolHandler) {
	inputSchema := mcpgo.ToolInputSchema{
		Type: "object",
	}

	if schema != nil {
		if props, ok := schema["properties"]; ok {
			propsJSON, _ := json.Marshal(props)
			json.Unmarshal(propsJSON, &inputSchema.Properties)
		}
		if req, ok := schema["required"].([]string); ok {
			inputSchema.Required = req
		}
	}

	tool := mcpgo.Tool{
		Name:        name,
		Description: description,
		InputSchema: inputSchema,
	}

	s.mcpServer.AddTool(tool, func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := make(map[string]interface{})
		if req.Params.Arguments != nil {
			if m, ok := req.Params.Arguments.(map[string]interface{}); ok {
				args = m
			}
		}

		result, err := handler(args)
		if err != nil {
			return mcpgo.NewToolResultError(err.Error()), nil
		}

		jsonBytes, _ := json.MarshalIndent(result, "", "  ")
		return mcpgo.NewToolResultText(string(jsonBytes)), nil
	})
}

func (s *Server) ListTools() []ToolInfo {
	tools := []ToolInfo{
		// Control plane
		{"auth-login", "Login to ModelsLab"},
		{"auth-signup", "Create account"},
		{"profile-get", "Get user profile"},
		{"profile-update", "Update profile"},
		{"api-keys-list", "List API keys"},
		{"api-keys-create", "Create API key"},
		{"models-search", "Search AI models"},
		{"models-detail", "Get model details"},
		{"billing-overview", "View billing overview"},
		{"wallet-balance", "Check wallet balance"},
		{"wallet-fund", "Add funds to wallet"},
		{"usage-summary", "Get usage overview"},
		{"teams-list", "List team members"},
		{"subscriptions-plans", "List subscription plans"},
		{"subscriptions-list", "List user subscriptions"},
		// Generation
		{"text-to-image", "Generate images from text"},
		{"image-to-image", "Transform images"},
		{"inpaint", "Inpaint images"},
		{"text-to-video", "Generate video from text"},
		{"image-to-video", "Animate image to video"},
		{"text-to-speech", "Convert text to speech"},
		{"speech-to-text", "Convert speech to text"},
		{"music-generation", "Generate music"},
		{"sound-effects", "Generate sound effects"},
		{"text-to-3d", "Generate 3D models"},
		{"chat-completion", "Chat completion"},
		{"fetch-image", "Check image generation status"},
		{"fetch-video", "Check video generation status"},
		{"fetch-audio", "Check audio generation status"},
		{"fetch-3d", "Check 3D generation status"},
	}
	return tools
}

func (s *Server) ServeStdio() error {
	return server.ServeStdio(s.mcpServer)
}

func (s *Server) ServeSSE(addr string) error {
	sseServer := server.NewSSEServer(s.mcpServer)
	fmt.Printf("MCP SSE server listening on %s\n", addr)
	return http.ListenAndServe(addr, sseServer)
}
