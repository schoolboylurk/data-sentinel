package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/permitio/permit-golang/pkg/enforcement"

	"github.com/schoolboylurk/data-sentinel/pkg/ai"
	"github.com/schoolboylurk/data-sentinel/pkg/auth"
	"github.com/schoolboylurk/data-sentinel/pkg/database"
)

// GenerateReportRequest is the JSON payload for direct AI processing by admins or AI-agents.
type GenerateReportRequest struct {
	Username string `json:"username"` // the calling user (should have process permission)
	Prompt   string `json:"prompt"`   // the raw prompt to wrap and send to AI
}

// GenerateReportHandler allows users with `prompt_requests.process` permission to directly invoke the AI.
func GenerateReportHandler(c *gin.Context) {
	var req GenerateReportRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Build enforcement objects
	user := enforcement.UserBuilder(req.Username).Build()
	resource := enforcement.ResourceBuilder("prompt_requests").Build()

	// Authorization: must have process permission
	allowed, err := auth.PermitClient.Check(user, "prompt_requests.process", resource)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "authorization error"})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": "permission denied"})
		return
	}

	// Wrap prompt with child's policy
	wrapped := WrapPromptWithPolicy(req.Username, req.Prompt)

	// Call AI
	answer, err := ai.GenerateReport(wrapped)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AI generation failed"})
		return
	}

	// Audit event for processing
	database.LogEvent("prompt_processed", req.Username)

	// Return AI's answer
	c.JSON(http.StatusOK, gin.H{"answer": answer, "processed_at": time.Now().Format(time.RFC3339)})
}
