/*
 * @Author: Vincent Yang
 * @Date: 2025-04-08 22:43:56
 * @LastEditors: Vincent Yang
 * @LastEditTime: 2025-04-09 15:44:44
 * @FilePath: /raycast2api/service/types.go
 * @Telegram: https://t.me/missuo
 * @GitHub: https://github.com/missuo
 *
 * Copyright © 2025 by Vincent, All Rights Reserved.
 */

package service

import (
	"encoding/json"
)

// OpenAIMessage represents a message in OpenAI format
type OpenAIMessage struct {
	Role    string      `json:"role"`    // "user", "assistant", or "system"
	Content interface{} `json:"content"` // Can be string or array
}

// RaycastMessage represents a message in Raycast format
type RaycastMessage struct {
	Author  string `json:"author"` // "user" or "assistant"
	Content struct {
		Text string `json:"text"`
	} `json:"content"`
}

// RaycastChatRequest represents a chat request to Raycast API
type RaycastChatRequest struct {
	AdditionalSystemInstructions string           `json:"additional_system_instructions"`
	Debug                        bool             `json:"debug"`
	Locale                       string           `json:"locale"`
	Messages                     []RaycastMessage `json:"messages"`
	Model                        string           `json:"model"`
	Provider                     string           `json:"provider"`
	Source                       string           `json:"source"`
	SystemInstruction            string           `json:"system_instruction"`
	Temperature                  float64          `json:"temperature"`
	ThreadID                     string           `json:"thread_id"`
	Tools                        []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"tools"`
}

// OpenAIChatRequest represents a chat request in OpenAI format
type OpenAIChatRequest struct {
	Messages    []OpenAIMessage        `json:"messages"`
	Model       string                 `json:"model"`
	Temperature float64                `json:"temperature,omitempty"`
	Stream      bool                   `json:"stream,omitempty"`
	System      string                 `json:"system,omitempty"`       // Optional system message
	MaxTokens   int                    `json:"max_tokens,omitempty"`   // Optional max tokens
	TopP        float64                `json:"top_p,omitempty"`        // Optional top_p value
	FrequencyPenalty float64           `json:"frequency_penalty,omitempty"` // Optional frequency penalty
	PresencePenalty float64            `json:"presence_penalty,omitempty"`  // Optional presence penalty
	Extra       map[string]interface{} `json:"-"`                      // Fields not explicitly defined above
}

// UnmarshalJSON custom unmarshaler to capture undefined fields
func (r *OpenAIChatRequest) UnmarshalJSON(data []byte) error {
	// Create a map to parse all fields
	var rawMap map[string]interface{}
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return err
	}
	
	// Initialize the Extra map
	r.Extra = make(map[string]interface{})
	
	// Extract known fields
	if v, ok := rawMap["messages"]; ok {
		messages, err := json.Marshal(v)
		if err != nil {
			return err
		}
		var m []OpenAIMessage
		if err := json.Unmarshal(messages, &m); err != nil {
			return err
		}
		r.Messages = m
		delete(rawMap, "messages")
	}
	
	if v, ok := rawMap["model"].(string); ok {
		r.Model = v
		delete(rawMap, "model")
	}
	
	if v, ok := rawMap["temperature"].(float64); ok {
		r.Temperature = v
		delete(rawMap, "temperature")
	}
	
	if v, ok := rawMap["stream"].(bool); ok {
		r.Stream = v
		delete(rawMap, "stream")
	}
	
	if v, ok := rawMap["system"].(string); ok {
		r.System = v
		r.Extra["system"] = v  // Also store in Extra for backward compatibility
		delete(rawMap, "system")
	}
	
	if v, ok := rawMap["max_tokens"].(float64); ok {
		r.MaxTokens = int(v)
		delete(rawMap, "max_tokens")
	}
	
	if v, ok := rawMap["top_p"].(float64); ok {
		r.TopP = v
		delete(rawMap, "top_p")
	}
	
	if v, ok := rawMap["frequency_penalty"].(float64); ok {
		r.FrequencyPenalty = v
		delete(rawMap, "frequency_penalty")
	}
	
	if v, ok := rawMap["presence_penalty"].(float64); ok {
		r.PresencePenalty = v
		delete(rawMap, "presence_penalty")
	}
	
	// Store any remaining fields in Extra
	for k, v := range rawMap {
		r.Extra[k] = v
	}
	
	return nil
}

// OpenAIChatResponse represents a chat response in OpenAI format
type OpenAIChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role        string   `json:"role"`
			Content     string   `json:"content"`
			Refusal     *string  `json:"refusal"`
			Annotations []string `json:"annotations"`
		} `json:"message"`
		Logprobs     *string `json:"logprobs"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens        int `json:"prompt_tokens"`
		CompletionTokens    int `json:"completion_tokens"`
		TotalTokens         int `json:"total_tokens"`
		PromptTokensDetails struct {
			CachedTokens int `json:"cached_tokens"`
			AudioTokens  int `json:"audio_tokens"`
		} `json:"prompt_tokens_details"`
		CompletionTokensDetails struct {
			ReasoningTokens          int `json:"reasoning_tokens"`
			AudioTokens              int `json:"audio_tokens"`
			AcceptedPredictionTokens int `json:"accepted_prediction_tokens"`
			RejectedPredictionTokens int `json:"rejected_prediction_tokens"`
		} `json:"completion_tokens_details"`
	} `json:"usage"`
	ServiceTier       string `json:"service_tier"`
	SystemFingerprint string `json:"system_fingerprint"`
}

// RaycastSSEData represents SSE data from Raycast
type RaycastSSEData struct {
	Text         string `json:"text,omitempty"`
	FinishReason string `json:"finish_reason,omitempty"`
}

// OpenAIModelResponse represents a model list response in OpenAI format
type OpenAIModelResponse struct {
	Object string `json:"object"`
	Data   []struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		OwnedBy string `json:"owned_by"`
	} `json:"data"`
}
