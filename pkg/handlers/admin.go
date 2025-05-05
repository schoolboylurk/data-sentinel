package handlers

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/permitio/permit-golang/pkg/models"
	csrf "github.com/utrack/gin-csrf"

	"github.com/schoolboylurk/data-sentinel/pkg/auth"
	"github.com/schoolboylurk/data-sentinel/pkg/database"
)

// ShowLogin renders the admin login page.
func ShowLogin(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", gin.H{
		"csrfToken": csrf.GetToken(c),
	})
}

// PerformLogin authenticates the admin user and starts a session.
func PerformLogin(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")
	// For demo purposes only: hardcoded credentials.
	if username == "admin" && password == "2025DEVChallenge" {
		sess := sessions.Default(c)
		sess.Set("user", username)
		sess.Save()

		ctx := context.Background()

		userCreate := *models.NewUserCreate(username)
		//add more metadata here eventually like setting email and username

		if _, err := auth.PermitClient.Api.Users.SyncUser(ctx, userCreate); err != nil {
			log.Printf("Permit SyncUser failed for %s: %v", username, err)
		}
		c.Redirect(http.StatusSeeOther, "/admin/kids")
		return
	}
	// Authentication failed
	c.HTML(http.StatusUnauthorized, "login.html", gin.H{"error": "invalid credentials", "csrfToken": csrf.GetToken(c)})
}

// AdminRequired is a middleware that ensures the user is logged in as admin.
func AdminRequired(c *gin.Context) {
	sess := sessions.Default(c)
	if sess.Get("user") == nil {
		c.Redirect(http.StatusSeeOther, "/login")
		c.Abort()
		return
	}
	c.Next()
}

// ViolationMetrics returns the count of policy violation attempts per kid in the last 24 hours.
func ViolationMetrics(c *gin.Context) {
	rows, err := database.DB.Query(`
		SELECT kid_username, COUNT(*) AS attempts
		FROM violation_attempts
		WHERE timestamp > datetime('now','-24 hours')
		GROUP BY kid_username
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query violations"})
		return
	}
	defer rows.Close()

	var metrics []gin.H
	for rows.Next() {
		var kid string
		var count int
		err := rows.Scan(&kid, &count)
		if err != nil {
			continue
		}
		metrics = append(metrics, gin.H{"kid": kid, "attempts": count})
	}

	c.JSON(http.StatusOK, metrics)
}

// ListKidsPage shows the list of kids.
func ListKidsPage(c *gin.Context) {
	rows, err := database.DB.Query("SELECT username, age FROM kids")
	if err != nil {
		c.HTML(http.StatusInternalServerError, "kids.html", gin.H{"error": "failed to load kids", "csrfToken": csrf.GetToken(c)})
		return
	}
	defer rows.Close()

	type Kid struct {
		Username string
		Age      int
	}
	var kids []Kid
	for rows.Next() {
		var k Kid
		if err := rows.Scan(&k.Username, &k.Age); err != nil {
			continue
		}
		kids = append(kids, k)
	}
	c.HTML(http.StatusOK, "kids.html", gin.H{"Kids": kids, "csrfToken": csrf.GetToken(c)})
}

// AddKid handles adding or updating a kid.
func AddKid(c *gin.Context) {
	username := c.PostForm("username")
	ageStr := c.PostForm("age")

	// Parse age
	ageInt, err := strconv.Atoi(ageStr)
	if err != nil || ageInt < 0 {
		c.HTML(http.StatusBadRequest, "kids.html", gin.H{
			"error": "Age must be a non-negative integer", "csrfToken": csrf.GetToken(c),
		})
		return
	}

	if _, err := database.DB.Exec(
		"INSERT OR REPLACE INTO kids(username, age) VALUES(?,?)",
		username, ageInt,
	); err != nil {
		log.Printf("AddKid: failed to save %s age %d: %v", username, ageInt, err)
		c.HTML(http.StatusInternalServerError, "kids.html", gin.H{
			"error": "Failed to save kid", "csrfToken": csrf.GetToken(c),
		})
		return
	}

	c.Redirect(http.StatusSeeOther, "/admin/kids")
}

// ListPoliciesPage shows content policies.
func ListPoliciesPage(c *gin.Context) {
	rows, err := database.DB.Query("SELECT kid_username, allowed, restricted FROM content_policies")
	if err != nil {
		c.HTML(http.StatusInternalServerError, "policies.html", gin.H{"error": "failed to load policies", "csrfToken": csrf.GetToken(c)})
		return
	}
	defer rows.Close()

	type Policy struct {
		Username   string
		Allowed    string
		Restricted string
	}
	var policies []Policy
	for rows.Next() {
		var p Policy
		if err := rows.Scan(&p.Username, &p.Allowed, &p.Restricted); err != nil {
			continue
		}
		policies = append(policies, p)
	}
	c.HTML(http.StatusOK, "policies.html", gin.H{"Policies": policies, "csrfToken": csrf.GetToken(c)})
}

func UpdatePolicy(c *gin.Context) {
	username := c.PostForm("username")
	allowed := c.PostForm("allowed")
	restricted := c.PostForm("restricted")

	if _, err := database.DB.Exec(
		"INSERT OR REPLACE INTO content_policies(kid_username, allowed, restricted) VALUES(?,?,?)",
		username, allowed, restricted,
	); err != nil {
		log.Printf("UpdatePolicy: failed to save policy for %s: %v", username, err)
		c.HTML(http.StatusInternalServerError, "policies.html", gin.H{"error": "failed to save policy"})
		return
	}

	c.Redirect(http.StatusSeeOther, "/admin/policies")
}

// ListRequestsPage shows all pending prompt requests.
func ListRequestsPage(c *gin.Context) {
	rows, err := database.DB.Query("SELECT id, kid_username, prompt, approved, created_at FROM prompt_requests ORDER BY created_at DESC")
	if err != nil {
		c.HTML(http.StatusInternalServerError, "requests.html", gin.H{"error": "failed to load requests", "csrfToken": csrf.GetToken(c)})
		return
	}
	defer rows.Close()

	type Req struct {
		ID        int
		Username  string
		Prompt    string
		Approved  bool
		CreatedAt string
	}
	var reqs []Req
	for rows.Next() {
		var r Req
		if err := rows.Scan(&r.ID, &r.Username, &r.Prompt, &r.Approved, &r.CreatedAt); err != nil {
			continue
		}
		reqs = append(reqs, r)
	}
	c.HTML(http.StatusOK, "requests.html", gin.H{"Requests": reqs, "csrfToken": csrf.GetToken(c)})
}

// ShowAdminDashboard renders the main admin dashboard UI.
func ShowAdminDashboard(c *gin.Context) {
	c.HTML(http.StatusOK, "admin_dashboard.html", gin.H{"csrfToken": csrf.GetToken(c)})
}

// MetricsHandler returns a breakdown of all audit_events in the last 24h.
func MetricsHandler(c *gin.Context) {
	rows, err := database.DB.Query(`
      SELECT event_type, COUNT(*) AS count
      FROM audit_events
      WHERE timestamp > datetime('now','-24 hours')
      GROUP BY event_type
    `)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query metrics"})
		return
	}
	defer rows.Close()

	var stats []gin.H
	for rows.Next() {
		var event string
		var cnt int
		if err := rows.Scan(&event, &cnt); err != nil {
			continue
		}
		stats = append(stats, gin.H{"event": event, "count": cnt})
	}
	c.JSON(http.StatusOK, stats)
}

// ListGroupsPage shows all RBAC groups.
func ListGroupsPage(c *gin.Context) {
	rows, err := database.DB.Query("SELECT id, name FROM groups")
	if err != nil {
		c.HTML(http.StatusInternalServerError, "groups.html", gin.H{"error": "failed to load groups", "csrfToken": csrf.GetToken(c)})
		return
	}
	defer rows.Close()

	type Group struct {
		ID   int
		Name string
	}
	var gs []Group
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Name); err != nil {
			continue
		}
		gs = append(gs, g)
	}
	c.HTML(http.StatusOK, "groups.html", gin.H{"Groups": gs, "csrfToken": csrf.GetToken(c)})
}

// AddGroup creates a new RBAC group.
func AddGroup(c *gin.Context) {
	name := strings.TrimSpace(c.PostForm("name"))
	if name == "" {
		c.HTML(http.StatusBadRequest, "groups.html", gin.H{"error": "Group name is required", "csrfToken": csrf.GetToken(c)})
		return
	}

	// prevent duplicates
	var exists bool
	if err := database.DB.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM groups WHERE name=?)", name,
	).Scan(&exists); err != nil {
		log.Printf("AddGroup: lookup failed for %s: %v", name, err)
		c.HTML(http.StatusInternalServerError, "groups.html", gin.H{"error": "DB error", "csrfToken": csrf.GetToken(c)})
		return
	}
	if exists {
		c.HTML(http.StatusConflict, "groups.html", gin.H{"error": "Group already exists", "csrfToken": csrf.GetToken(c)})
		return
	}

	if _, err := database.DB.Exec("INSERT INTO groups(name) VALUES(?)", name); err != nil {
		log.Printf("AddGroup: failed to create group %s: %v", name, err)
		c.HTML(http.StatusInternalServerError, "groups.html", gin.H{"error": "Could not create group", "csrfToken": csrf.GetToken(c)})
		return
	}

	c.Redirect(http.StatusSeeOther, "/admin/groups")
}

// AddMember adds a user to an existing group.
func AddMember(c *gin.Context) {
	gid, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.HTML(http.StatusBadRequest, "groups.html", gin.H{"error": "Invalid group ID", "csrfToken": csrf.GetToken(c)})
		return
	}
	user := strings.TrimSpace(c.PostForm("username"))
	if user == "" {
		c.HTML(http.StatusBadRequest, "groups.html", gin.H{"error": "Username is required", "csrfToken": csrf.GetToken(c)})
		return
	}

	// prevent duplicate membership
	var exists bool
	if err := database.DB.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM group_members WHERE group_id=? AND username=?)",
		gid, user,
	).Scan(&exists); err != nil {
		log.Printf("AddMember: lookup failed for group %d, user %s: %v", gid, user, err)
		c.HTML(http.StatusInternalServerError, "groups.html", gin.H{"error": "DB error", "csrfToken": csrf.GetToken(c)})
		return
	}
	if exists {
		c.HTML(http.StatusConflict, "groups.html", gin.H{"error": "User already in group", "csrfToken": csrf.GetToken(c)})
		return
	}

	if _, err := database.DB.Exec(
		"INSERT INTO group_members(group_id, username) VALUES(?,?)",
		gid, user,
	); err != nil {
		log.Printf("AddMember: failed to add %s to group %d: %v", user, gid, err)
		c.HTML(http.StatusInternalServerError, "groups.html", gin.H{"error": "Could not add member", "csrfToken": csrf.GetToken(c)})
		return
	}

	c.Redirect(http.StatusSeeOther, "/admin/groups")
}
