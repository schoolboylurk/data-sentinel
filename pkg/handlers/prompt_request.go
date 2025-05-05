package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/permitio/permit-golang/pkg/enforcement"

	"github.com/schoolboylurk/data-sentinel/pkg/ai"
	"github.com/schoolboylurk/data-sentinel/pkg/auth"
	"github.com/schoolboylurk/data-sentinel/pkg/database"
)

const MaxPromptLength = 1000

// PromptRequest represents a child's prompt submission JSON payload.
type PromptRequest struct {
	Username string `json:"username"`
	Prompt   string `json:"prompt"`
}

// RequestPromptHandler logs a new prompt request (pending approval).
// Enforces that the child has permission to create prompt_requests.
func RequestPromptHandler(c *gin.Context) {
	var req PromptRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.Prompt) > MaxPromptLength {
		c.JSON(http.StatusBadRequest, gin.H{"	error": fmt.Sprintf("Prompt too long (max %d characters)", MaxPromptLength)})
		return
	}

	// Build enforcement objects
	user := enforcement.UserBuilder(req.Username).Build()
	resource := enforcement.ResourceBuilder("prompt_requests").Build()

	// Authorization: only children can create prompt requests
	allowed, err := auth.PermitClient.Check(user, "prompt_requests.create", resource)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "authorization error"})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": "permission denied"})
		return
	}

	// Insert new request into DB, with error logging
	res, err := database.DB.Exec(
		"INSERT INTO prompt_requests(kid_username, prompt, created_at) VALUES(?,?,?)",
		req.Username, req.Prompt, time.Now(),
	)
	if err != nil {
		log.Printf("⚠️ RequestPromptHandler: failed to insert prompt request for user %s: %v", req.Username, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not save prompt request"})
		return
	}
	id, _ := res.LastInsertId()

	// Audit event (also log errors internally if needed)
	if err := database.LogEvent("prompt_submitted", req.Username); err != nil {
		log.Printf("⚠️ RequestPromptHandler: failed to log event for user %s: %v", req.Username, err)
	}

	// Return success to client
	c.JSON(http.StatusCreated, gin.H{"request_id": id, "status": "pending"})
}

// ApprovePromptHandler allows parents (admins) to approve a child's prompt and generate an AI response.
func ApprovePromptHandler(c *gin.Context) {
	admin := c.Query("username")
	idParam := c.Param("id")
	id, err := strconv.Atoi(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request ID"})
		return
	}

	// Build enforcement objects
	user := enforcement.UserBuilder(admin).Build()
	resource := enforcement.ResourceBuilder("prompt_requests").Build()

	// Authorization: only admins can approve
	allowed, err := auth.PermitClient.Check(user, "prompt_requests.approve", resource)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "authorization error"})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": "permission denied"})
		return
	}

	// Mark approved in DB
	_, err = database.DB.Exec("UPDATE prompt_requests SET approved = TRUE WHERE id = ?", id)
	if err != nil {
		log.Printf("⚠️ ApprovePromptHandler: failed to update prompt_requests id=%d: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db update failed"})
		return
	}

	// Fetch the original request
	var kid, userPrompt string
	if err := database.DB.QueryRow(
		"SELECT kid_username, prompt FROM prompt_requests WHERE id = ?", id,
	).Scan(&kid, &userPrompt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "db lookup failed"})
		return
	}

	// Build prompt with policy and generate response
	wrapped := WrapPromptWithPolicy(kid, userPrompt)
	answer, err := ai.GenerateReport(wrapped)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AI generation failed"})
		return
	}

	// Audit event for approval
	if err := database.LogEvent("prompt_approved", admin); err != nil {
		log.Printf("⚠️ ApprovePromptHandler: failed to log event for admin %s: %v", admin, err)
	}

	c.JSON(http.StatusOK, gin.H{
		"request_id": id,
		"approved":   true,
		"answer":     answer,
		"timestamp":  time.Now().Format(time.RFC3339),
	})
}
