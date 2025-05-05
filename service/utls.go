/*
 * @Author: Vincent Yang
 * @Date: 2025-04-08 22:44:55
 * @LastEditors: Vincent Yang
 * @LastEditTime: 2025-04-09 15:39:59
 * @FilePath: /raycast2api/service/utls.go
 * @Telegram: https://t.me/missuo
 * @GitHub: https://github.com/missuo
 *
 * Copyright © 2025 by Vincent, All Rights Reserved.
 */

package service

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// convertMessages converts OpenAI messages format to Raycast format
func convertMessages(openaiMessages []OpenAIMessage) []RaycastMessage {
	raycastMessages := make([]RaycastMessage, len(openaiMessages))
	for i, msg := range openaiMessages {
		author := "user"
		if msg.Role == "assistant" {
			author = "assistant"
		}

		var contentText string
		switch content := msg.Content.(type) {
		case string:
			contentText = content
		case []interface{}:
			// Handle array content (extract text parts)
			for _, part := range content {
				if partMap, ok := part.(map[string]interface{}); ok {
					if partMap["type"] == "text" {
						if textValue, ok := partMap["text"].(string); ok {
							contentText += textValue
						}
					}
				}
			}
		}

		raycastMessages[i] = RaycastMessage{
			Author: author,
			Content: struct {
				Text string `json:"text"`
			}{
				Text: contentText,
			},
		}
	}
	return raycastMessages
}

// parseSSEResponse parses SSE response from Raycast into a single text
func parseSSEResponse(responseText string) string {
	scanner := bufio.NewScanner(strings.NewReader(responseText))
	var fullText string
	
	log.Printf("Starting to parse SSE response, length: %d", len(responseText))
	
	// If the response is empty, return early
	if strings.TrimSpace(responseText) == "" {
		log.Println("Empty response received from Raycast")
		return ""
	}

	lineCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		lineCount++
		
		if line == "" {
			continue
		}
		
		if strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			log.Printf("SSE data line %d: %s", lineCount, data)
			
			// Skip [DONE] marker
			if data == "[DONE]" {
				log.Println("Reached end of SSE stream")
				continue
			}
			
			// Try standard parsing first
			var jsonData RaycastSSEData
			if err := json.Unmarshal([]byte(data), &jsonData); err != nil {
				log.Printf("Failed to parse SSE data as RaycastSSEData: %v", err)
				
				// If standard parsing fails, try as a generic JSON object
				var genericData map[string]interface{}
				if jsonErr := json.Unmarshal([]byte(data), &genericData); jsonErr != nil {
					log.Printf("Failed to parse as generic JSON: %v", jsonErr)
					continue
				}
				
				// Try to extract text from various possible fields
				if text, ok := genericData["text"].(string); ok && text != "" {
					log.Printf("Found text in generic JSON: %s", text)
					fullText += text
					continue
				}
				
				if content, ok := genericData["content"].(string); ok && content != "" {
					log.Printf("Found content in generic JSON: %s", content)
					fullText += content
					continue
				}
				
				// Check for nested message structure
				if message, ok := genericData["message"].(map[string]interface{}); ok {
					if content, ok := message["content"].(string); ok && content != "" {
						log.Printf("Found content in nested message: %s", content)
						fullText += content
						continue
					}
				}
				
				// If we got here, we found JSON but no recognizable text field
				log.Printf("Found JSON but no text/content fields: %v", genericData)
				continue
			}
			
			// Standard parsing succeeded
			if jsonData.Text != "" {
				log.Printf("Adding text from standard format: %s", jsonData.Text)
				fullText += jsonData.Text
			} else {
				log.Println("Empty text field in otherwise valid JSON")
			}
		} else {
			// Log non-data lines for debugging
			log.Printf("Non-data line: %s", line)
		}
	}
	
	log.Printf("Parsed response, extracted text length: %d", len(fullText))
	
	// If we didn't extract any text but had data lines, try one more fallback approach
	if fullText == "" && lineCount > 0 {
		log.Println("No text extracted but response exists, trying whole-response parsing")
		
		// Try to extract any JSON objects from the entire response
		var allMatches []string
		re := regexp.MustCompile(`{[^{}]*({[^{}]*})*[^{}]*}`)
		matches := re.FindAllString(responseText, -1)
		
		for _, match := range matches {
			var genericData map[string]interface{}
			if err := json.Unmarshal([]byte(match), &genericData); err == nil {
				// Look for content or text fields at any level (simplified)
				jsonBytes, _ := json.Marshal(genericData)
				if strings.Contains(string(jsonBytes), "\"text\":") || 
				   strings.Contains(string(jsonBytes), "\"content\":") {
					allMatches = append(allMatches, match)
				}
			}
		}
		
		if len(allMatches) > 0 {
			log.Printf("Found %d potential JSON objects in response", len(allMatches))
			// For now just log them, could add more parsing logic here
		}
	}

	return fullText
}

// handleStreamingResponse handles streaming response from Raycast
func handleStreamingResponse(c *gin.Context, response *http.Response, modelId string) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(http.StatusOK)

	// Set up a flush interval for the writer
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		log.Println("Streaming unsupported")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	reader := bufio.NewReader(response.Body)
	buffer := ""

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("Error reading from response: %v", err)
			break
		}

		buffer += line

		// Process complete SSE messages in the buffer
		if strings.HasSuffix(buffer, "\n\n") {
			lines := strings.Split(buffer, "\n")
			buffer = ""

			for _, l := range lines {
				if strings.TrimSpace(l) == "" {
					continue
				}

				if strings.HasPrefix(l, "data:") {
					data := strings.TrimSpace(strings.TrimPrefix(l, "data:"))
					var jsonData RaycastSSEData
					if err := json.Unmarshal([]byte(data), &jsonData); err != nil {
						log.Printf("Failed to parse SSE data: %v", err)
						continue
					}

					// Create OpenAI-compatible streaming chunk
					chunk := struct {
						ID      string `json:"id"`
						Object  string `json:"object"`
						Created int64  `json:"created"`
						Model   string `json:"model"`
						Choices []struct {
							Index int `json:"index"`
							Delta struct {
								Content string `json:"content"`
							} `json:"delta"`
							FinishReason string `json:"finish_reason"`
						} `json:"choices"`
					}{
						ID:      fmt.Sprintf("chatcmpl-%s", uuid.New().String()),
						Object:  "chat.completion.chunk",
						Created: time.Now().Unix(),
						Model:   modelId,
						Choices: []struct {
							Index int `json:"index"`
							Delta struct {
								Content string `json:"content"`
							} `json:"delta"`
							FinishReason string `json:"finish_reason"`
						}{
							{
								Index: 0,
								Delta: struct {
									Content string `json:"content"`
								}{
									Content: jsonData.Text,
								},
								FinishReason: jsonData.FinishReason,
							},
						},
					}

					chunkData, err := json.Marshal(chunk)
					if err != nil {
						log.Printf("Error marshaling chunk: %v", err)
						continue
					}

					// Send the chunk
					fmt.Fprintf(c.Writer, "data: %s\n\n", string(chunkData))
					flusher.Flush()
				}
			}
		}
	}

	// Send final [DONE] marker
	fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
	flusher.Flush()
}

// handleNonStreamingResponse handles non-streaming response from Raycast
func handleNonStreamingResponse(c *gin.Context, response *http.Response, modelId string) {
	// Collect the entire response
	bodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: struct {
				Message string `json:"message"`
				Type    string `json:"type"`
				Details string `json:"details,omitempty"`
			}{
				Message: "Error reading response body",
				Type:    "server_error",
				Details: err.Error(),
			},
		})
		return
	}

	responseText := string(bodyBytes)
	log.Printf("Raw response: %s", responseText)

	// Parse the SSE format to extract the full text
	fullText := parseSSEResponse(responseText)
	
	// If no text was extracted, try direct JSON parsing as fallback
	if fullText == "" {
		log.Println("No text extracted from SSE parsing, trying direct JSON parsing")
		
		// First, check if the response is a complete JSON object
		var directJsonResponse map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &directJsonResponse); err == nil {
			log.Println("Response is a valid JSON object, checking for content")
			
			// Check for various content fields
			if extractedText := extractTextFromJSON(directJsonResponse); extractedText != "" {
				log.Printf("Extracted text directly from JSON: %s", extractedText)
				fullText = extractedText
			}
		}
		
		// If still no content, use a default message to indicate the issue
		if fullText == "" {
			log.Println("Warning: Could not extract any content from response")
			fullText = "抱歉，无法提取响应内容。请尝试重新发送请求或联系管理员检查服务器日志。"
		}
	}

	// Convert to OpenAI format
	openaiResponse := OpenAIChatResponse{
		ID:      fmt.Sprintf("chatcmpl-%s", uuid.New().String()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   modelId,
		Choices: []struct {
			Index   int `json:"index"`
			Message struct {
				Role        string   `json:"role"`
				Content     string   `json:"content"`
				Refusal     *string  `json:"refusal"`
				Annotations []string `json:"annotations"`
			} `json:"message"`
			Logprobs     *string `json:"logprobs"`
			FinishReason string  `json:"finish_reason"`
		}{
			{
				Index: 0,
				Message: struct {
					Role        string   `json:"role"`
					Content     string   `json:"content"`
					Refusal     *string  `json:"refusal"`
					Annotations []string `json:"annotations"`
				}{
					Role:        "assistant",
					Content:     fullText,
					Refusal:     nil,
					Annotations: []string{},
				},
				Logprobs:     nil,
				FinishReason: "length",
			},
		},
		Usage: struct {
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
		}{
			PromptTokens:     10,
			CompletionTokens: 10,
			TotalTokens:      20,
			PromptTokensDetails: struct {
				CachedTokens int `json:"cached_tokens"`
				AudioTokens  int `json:"audio_tokens"`
			}{
				CachedTokens: 0,
				AudioTokens:  0,
			},
			CompletionTokensDetails: struct {
				ReasoningTokens          int `json:"reasoning_tokens"`
				AudioTokens              int `json:"audio_tokens"`
				AcceptedPredictionTokens int `json:"accepted_prediction_tokens"`
				RejectedPredictionTokens int `json:"rejected_prediction_tokens"`
			}{
				ReasoningTokens:          0,
				AudioTokens:              0,
				AcceptedPredictionTokens: 0,
				RejectedPredictionTokens: 0,
			},
		},
		ServiceTier:       "default",
		SystemFingerprint: "fp_b376dfbbd5",
	}

	jsonData, err := json.MarshalIndent(openaiResponse, "", "  ")
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error: struct {
				Message string `json:"message"`
				Type    string `json:"type"`
				Details string `json:"details,omitempty"`
			}{
				Message: "Error formatting JSON response",
				Type:    "server_error",
				Details: err.Error(),
			},
		})
		return
	}

	// Add a newline to the end of the JSON data
	jsonData = append(jsonData, '\n')
	// Set content type and write the formatted JSON
	c.Header("Content-Type", "application/json")
	c.Writer.Write(jsonData)
}

// Add a helper function to extract text from JSON
func extractTextFromJSON(jsonData map[string]interface{}) string {
	// Check for common patterns in the JSON response
	
	// Pattern 1: Direct content field
	if content, ok := jsonData["content"].(string); ok && content != "" {
		return content
	}
	
	// Pattern 2: Check message structure
	if choices, ok := jsonData["choices"].([]interface{}); ok && len(choices) > 0 {
		// Try to extract from first choice
		if choice, ok := choices[0].(map[string]interface{}); ok {
			// Check for message
			if message, ok := choice["message"].(map[string]interface{}); ok {
				if content, ok := message["content"].(string); ok && content != "" {
					return content
				}
			}
			
			// Check for direct delta content
			if delta, ok := choice["delta"].(map[string]interface{}); ok {
				if content, ok := delta["content"].(string); ok && content != "" {
					return content
				}
			}
		}
	}
	
	// Pattern 3: Check for text field at top level
	if text, ok := jsonData["text"].(string); ok && text != "" {
		return text
	}
	
	// Pattern 4: Check for completion field (some APIs use this)
	if completion, ok := jsonData["completion"].(string); ok && completion != "" {
		return completion
	}
	
	// No content found
	return ""
}
