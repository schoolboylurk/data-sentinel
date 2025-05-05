package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/permitio/permit-golang/pkg/models"
	csrf "github.com/utrack/gin-csrf"

	"github.com/schoolboylurk/data-sentinel/pkg/ai"
	"github.com/schoolboylurk/data-sentinel/pkg/auth"
	"github.com/schoolboylurk/data-sentinel/pkg/database"
)

// ChildRequired middleware ensures the kid is logged in
func ChildRequired(c *gin.Context) {
	sess := sessions.Default(c)
	if sess.Get("kid") == nil {
		c.Redirect(http.StatusSeeOther, "/child/login")
		c.Abort()
		return
	}
	c.Next()
}

// ShowChildLogin renders the login page for kids.
func ShowChildLogin(c *gin.Context) {
	c.HTML(http.StatusOK, "child_login.html", gin.H{"csrfToken": csrf.GetToken(c)})
}

// PerformChildLogin authenticates a kid by username.
func PerformChildLogin(c *gin.Context) {
	username := c.PostForm("username")

	var age int
	if err := database.DB.QueryRow(
		"SELECT age FROM kids WHERE username = ?", username,
	).Scan(&age); err != nil {
		log.Printf("PerformChildLogin: unknown kid %s: %v", username, err)
		c.HTML(http.StatusUnauthorized, "child_login.html", gin.H{"error": "invalid username", "csrfToken": csrf.GetToken(c)})
		return
	}

	sess := sessions.Default(c)
	sess.Set("kid", username)
	sess.Save()

	ctx := context.Background()

	userCreate := *models.NewUserCreate(username)
	//add more metadata here eventually like setting age and username

	if _, err := auth.PermitClient.Api.Users.SyncUser(ctx, userCreate); err != nil {
		log.Printf("Permit SyncUser failed for %s: %v", username, err)
	}
	c.Redirect(http.StatusSeeOther, "/child/chat")
}

// ShowChildPromptPage is the legacy single‐prompt UI.
func ShowChildPromptPage(c *gin.Context) {
	c.HTML(http.StatusOK, "child.html", gin.H{"csrfToken": csrf.GetToken(c)})
}

// HandleChildPrompt handles that single‐prompt flow.
func HandleChildPrompt(c *gin.Context) {
	// invoke RequestPromptHandler logic,
	// then redirect to /child/status/:id
}

// ShowChildStatusPage renders status/answer for a single prompt.
func ShowChildStatusPage(c *gin.Context) {
	// query prompt_requests for this kid and ID,
	// if approved render answer via "child_status.html"
}

// ShowChildChatPage renders the persistent chat UI.
func ShowChildChatPage(c *gin.Context) {
	c.HTML(http.StatusOK, "child_chat.html", gin.H{"csrfToken": csrf.GetToken(c)})
}

// StartChatSession creates a new chat session for the logged-in kid.
func StartChatSession(c *gin.Context) {
	kid := sessions.Default(c).Get("kid").(string)

	res, err := database.DB.Exec("INSERT INTO chat_sessions(kid_username) VALUES(?)", kid)
	if err != nil {
		log.Printf("StartChatSession: failed for %s: %v", kid, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start session"})
		return
	}
	sid, _ := res.LastInsertId()
	c.JSON(http.StatusCreated, gin.H{"session_id": sid})
}

// PostMessage handles a kid’s message, enforces policy, calls AI, and records both sides.
func PostMessage(c *gin.Context) {
	sidParam := c.Param("id")
	sid, err := strconv.Atoi(sidParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid session ID"})
		return
	}

	var body struct {
		Content string `json:"content"`
	}
	if err := c.BindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(body.Content) > MaxPromptLength {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Message too long (max %d chars)", MaxPromptLength),
		})
		return
	}

	kid := sessions.Default(c).Get("kid").(string)
	wrapped := WrapPromptWithPolicy(kid, body.Content)

	// save kid’s message
	if _, err := database.DB.Exec(
		"INSERT INTO chat_messages(session_id,sender,content) VALUES(?,?,?)",
		sid, "kid", body.Content,
	); err != nil {
		log.Printf("⚠️ PostMessage: failed to save kid msg: %v", err)
	}

	// call AI
	answer, err := ai.GenerateReport(wrapped)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AI error"})
		return
	}

	// save AI response
	if _, err := database.DB.Exec(
		"INSERT INTO chat_messages(session_id,sender,content) VALUES(?,?,?)",
		sid, "ai", answer,
	); err != nil {
		log.Printf("⚠️ PostMessage: failed to save AI msg: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{"answer": answer})
}

// GetChatHistory returns the full chat history for a session.
func GetChatHistory(c *gin.Context) {
	sidParam := c.Param("id")
	sid, err := strconv.Atoi(sidParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid session ID"})
		return
	}

	rows, err := database.DB.Query(
		"SELECT sender,content,timestamp FROM chat_messages WHERE session_id = ? ORDER BY id",
		sid,
	)
	if err != nil {
		log.Printf("⚠️ GetChatHistory: query failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch history"})
		return
	}
	defer rows.Close()

	var msgs []gin.H
	for rows.Next() {
		var sender, content, ts string
		if err := rows.Scan(&sender, &content, &ts); err != nil {
			log.Printf("⚠️ GetChatHistory: scan failed: %v", err)
			continue
		}
		msgs = append(msgs, gin.H{"sender": sender, "content": content, "timestamp": ts})
	}

	c.JSON(http.StatusOK, msgs)
}
