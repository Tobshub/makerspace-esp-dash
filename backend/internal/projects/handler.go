package projects

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/access"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/auth"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/httpapi"
)

func Register(r *gin.RouterGroup, pool *pgxpool.Pool) {
	r.GET("/teams/:teamId/projects", func(c *gin.Context) { handleList(c, pool) })
	r.POST("/teams/:teamId/projects", func(c *gin.Context) { handleCreate(c, pool) })
	r.GET("/projects/:projectId", func(c *gin.Context) { handleGet(c, pool) })
	r.PATCH("/projects/:projectId", func(c *gin.Context) { handleUpdate(c, pool) })
	r.DELETE("/projects/:projectId", func(c *gin.Context) { handleDelete(c, pool) })
}

type projectBody struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func handleList(c *gin.Context, pool *pgxpool.Pool) {
	if !httpapi.ValidID(c.Param("teamId")) {
		httpapi.Error(c, http.StatusNotFound, "NOT_FOUND", "Not found")
		return
	}
	items, err := listProjects(c.Request.Context(), pool, auth.Current(c).ID, c.Param("teamId"))
	if httpapi.WriteRead(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"projects": items})
}

func handleCreate(c *gin.Context, pool *pgxpool.Pool) {
	if !httpapi.ValidID(c.Param("teamId")) {
		httpapi.Error(c, http.StatusNotFound, "NOT_FOUND", "Not found")
		return
	}
	role, err := access.TeamRole(c.Request.Context(), pool, auth.Current(c).ID, c.Param("teamId"))
	if httpapi.WriteRead(c, err) {
		return
	}
	if !access.CanWrite(role) {
		httpapi.Forbidden(c)
		return
	}
	name, description, ok := readProjectBody(c)
	if !ok {
		return
	}
	item, err := createProject(c.Request.Context(), pool, auth.Current(c).ID, c.Param("teamId"), name, description)
	if httpapi.WriteRead(c, err) {
		return
	}
	c.JSON(http.StatusCreated, gin.H{"project": item})
}

func handleGet(c *gin.Context, pool *pgxpool.Pool) {
	if !httpapi.ValidID(c.Param("projectId")) {
		httpapi.Error(c, http.StatusNotFound, "NOT_FOUND", "Not found")
		return
	}
	item, err := getProject(c.Request.Context(), pool, auth.Current(c).ID, c.Param("projectId"))
	if httpapi.WriteRead(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"project": item})
}

func handleUpdate(c *gin.Context, pool *pgxpool.Pool) {
	if !httpapi.ValidID(c.Param("projectId")) {
		httpapi.Error(c, http.StatusNotFound, "NOT_FOUND", "Not found")
		return
	}
	membership, err := access.Project(c.Request.Context(), pool, auth.Current(c).ID, c.Param("projectId"))
	if httpapi.WriteRead(c, err) {
		return
	}
	if !access.CanWrite(membership.Role) {
		httpapi.Forbidden(c)
		return
	}
	name, description, ok := readProjectBody(c)
	if !ok {
		return
	}
	item, err := updateProject(c.Request.Context(), pool, auth.Current(c).ID, c.Param("projectId"), name, description)
	if httpapi.WriteRead(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"project": item})
}

func handleDelete(c *gin.Context, pool *pgxpool.Pool) {
	if !httpapi.ValidID(c.Param("projectId")) {
		httpapi.Error(c, http.StatusNotFound, "NOT_FOUND", "Not found")
		return
	}
	membership, err := access.Project(c.Request.Context(), pool, auth.Current(c).ID, c.Param("projectId"))
	if httpapi.WriteRead(c, err) {
		return
	}
	if !access.CanManage(membership.Role) {
		httpapi.Error(c, http.StatusForbidden, "FORBIDDEN", "Only an owner or admin can delete a project")
		return
	}
	if err := deleteProject(c.Request.Context(), pool, auth.Current(c).ID, c.Param("projectId")); httpapi.WriteRead(c, err) {
		return
	}
	c.Status(http.StatusNoContent)
}

func readProjectBody(c *gin.Context) (string, string, bool) {
	var body projectBody
	if c.ShouldBindJSON(&body) != nil {
		httpapi.Validation(c, map[string]string{"body": "Request must be JSON"})
		return "", "", false
	}
	name := strings.TrimSpace(body.Name)
	description := strings.TrimSpace(body.Description)
	fields := map[string]string{}
	if name == "" || len(name) > 80 {
		fields["name"] = "Name is required"
	}
	if len(description) > 2000 {
		fields["description"] = "Description is too long"
	}
	if len(fields) > 0 {
		httpapi.Validation(c, fields)
		return "", "", false
	}
	return name, description, true
}
