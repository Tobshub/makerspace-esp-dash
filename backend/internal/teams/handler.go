package teams

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Tobshub/makerspace-esp-dash/backend/internal/access"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/auth"
	"github.com/Tobshub/makerspace-esp-dash/backend/internal/httpapi"
)

func Register(r *gin.RouterGroup, pool *pgxpool.Pool) {
	r.GET("/teams", func(c *gin.Context) { handleList(c, pool) })
	r.POST("/teams", func(c *gin.Context) { handleCreate(c, pool) })
	r.GET("/teams/:teamId", func(c *gin.Context) { handleGet(c, pool) })
	r.PATCH("/teams/:teamId", func(c *gin.Context) { handleUpdate(c, pool) })
	r.DELETE("/teams/:teamId", func(c *gin.Context) { handleDelete(c, pool) })
	r.GET("/teams/:teamId/members", func(c *gin.Context) { handleListMembers(c, pool) })
	r.POST("/teams/:teamId/members", func(c *gin.Context) { handleAddMember(c, pool) })
	r.PATCH("/teams/:teamId/members/:memberId", func(c *gin.Context) { handleUpdateMember(c, pool) })
	r.DELETE("/teams/:teamId/members/:memberId", func(c *gin.Context) { handleRemoveMember(c, pool) })
}

type nameBody struct {
	Name string `json:"name"`
}

type memberBody struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

func handleList(c *gin.Context, pool *pgxpool.Pool) {
	teams, err := listTeams(c.Request.Context(), pool, auth.Current(c).ID)
	if err != nil {
		httpapi.Internal(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"teams": teams})
}

func handleCreate(c *gin.Context, pool *pgxpool.Pool) {
	var body nameBody
	if c.ShouldBindJSON(&body) != nil {
		httpapi.Validation(c, map[string]string{"body": "Request must be JSON"})
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" || len(name) > 80 {
		httpapi.Validation(c, map[string]string{"name": "Name is required"})
		return
	}
	team, err := createTeam(c.Request.Context(), pool, auth.Current(c).ID, name)
	if err != nil {
		httpapi.Internal(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"team": team})
}

func handleGet(c *gin.Context, pool *pgxpool.Pool) {
	if !httpapi.ValidID(c.Param("teamId")) {
		httpapi.Error(c, http.StatusNotFound, "NOT_FOUND", "Not found")
		return
	}
	team, err := getTeam(c.Request.Context(), pool, auth.Current(c).ID, c.Param("teamId"))
	if httpapi.WriteRead(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"team": team})
}

func handleUpdate(c *gin.Context, pool *pgxpool.Pool) {
	if !httpapi.ValidID(c.Param("teamId")) {
		httpapi.Error(c, http.StatusNotFound, "NOT_FOUND", "Not found")
		return
	}
	role, err := access.TeamRole(c.Request.Context(), pool, auth.Current(c).ID, c.Param("teamId"))
	if httpapi.WriteRead(c, err) {
		return
	}
	if !access.CanManage(role) {
		httpapi.Forbidden(c)
		return
	}
	var body nameBody
	if c.ShouldBindJSON(&body) != nil {
		httpapi.Validation(c, map[string]string{"body": "Request must be JSON"})
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" || len(name) > 80 {
		httpapi.Validation(c, map[string]string{"name": "Name is required"})
		return
	}
	team, err := updateTeam(c.Request.Context(), pool, auth.Current(c).ID, c.Param("teamId"), name)
	if httpapi.WriteRead(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"team": team})
}

func handleDelete(c *gin.Context, pool *pgxpool.Pool) {
	if !httpapi.ValidID(c.Param("teamId")) {
		httpapi.Error(c, http.StatusNotFound, "NOT_FOUND", "Not found")
		return
	}
	err := deleteTeam(c.Request.Context(), pool, auth.Current(c).ID, c.Param("teamId"))
	if errors.Is(err, errNotOwner) {
		httpapi.Error(c, http.StatusForbidden, "FORBIDDEN", "Only an owner can delete a team")
		return
	}
	if httpapi.WriteRead(c, err) {
		return
	}
	c.Status(http.StatusNoContent)
}

func handleListMembers(c *gin.Context, pool *pgxpool.Pool) {
	if !httpapi.ValidID(c.Param("teamId")) {
		httpapi.Error(c, http.StatusNotFound, "NOT_FOUND", "Not found")
		return
	}
	if _, err := access.TeamRole(c.Request.Context(), pool, auth.Current(c).ID, c.Param("teamId")); httpapi.WriteRead(c, err) {
		return
	}
	members, err := listMembers(c.Request.Context(), pool, c.Param("teamId"))
	if err != nil {
		httpapi.Internal(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"members": members})
}

func handleAddMember(c *gin.Context, pool *pgxpool.Pool) {
	if !requireManage(c, pool) {
		return
	}
	var body memberBody
	if c.ShouldBindJSON(&body) != nil {
		httpapi.Validation(c, map[string]string{"body": "Request must be JSON"})
		return
	}
	role := body.Role
	if role == "" {
		role = access.RoleMember
	}
	if !access.ValidRole(role) {
		httpapi.Validation(c, map[string]string{"role": "Role must be owner, admin, member, or viewer"})
		return
	}
	actor, _ := access.TeamRole(c.Request.Context(), pool, auth.Current(c).ID, c.Param("teamId"))
	if role == access.RoleOwner && actor != access.RoleOwner {
		httpapi.Error(c, http.StatusForbidden, "FORBIDDEN", "Only an owner can add another owner")
		return
	}
	if strings.TrimSpace(body.Email) == "" {
		httpapi.Validation(c, map[string]string{"email": "Email is required"})
		return
	}
	member, err := addMember(c.Request.Context(), pool, c.Param("teamId"), body.Email, role)
	if errors.Is(err, ErrUserNotFound) {
		httpapi.Validation(c, map[string]string{"email": "No account uses that email. They need to sign up first."})
		return
	}
	if errors.Is(err, ErrConflict) {
		httpapi.Error(c, http.StatusConflict, "CONFLICT", "That person is already on the team")
		return
	}
	if err != nil {
		httpapi.Internal(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"member": member})
}

func handleUpdateMember(c *gin.Context, pool *pgxpool.Pool) {
	if !requireManage(c, pool) {
		return
	}
	if !httpapi.ValidID(c.Param("memberId")) {
		httpapi.Error(c, http.StatusNotFound, "NOT_FOUND", "Not found")
		return
	}
	var body memberBody
	if c.ShouldBindJSON(&body) != nil {
		httpapi.Validation(c, map[string]string{"body": "Request must be JSON"})
		return
	}
	if !access.ValidRole(body.Role) {
		httpapi.Validation(c, map[string]string{"role": "Role must be owner, admin, member, or viewer"})
		return
	}
	current, err := memberRole(c.Request.Context(), pool, c.Param("teamId"), c.Param("memberId"))
	if httpapi.WriteRead(c, err) {
		return
	}
	actor, _ := access.TeamRole(c.Request.Context(), pool, auth.Current(c).ID, c.Param("teamId"))
	if (current == access.RoleOwner || body.Role == access.RoleOwner) && actor != access.RoleOwner {
		httpapi.Error(c, http.StatusForbidden, "FORBIDDEN", "Only an owner can change the owner role")
		return
	}
	member, err := updateMemberRole(c.Request.Context(), pool, c.Param("teamId"), c.Param("memberId"), body.Role)
	if errors.Is(err, ErrLastOwner) {
		httpapi.Validation(c, map[string]string{"role": "The team needs an owner"})
		return
	}
	if httpapi.WriteRead(c, err) {
		return
	}
	c.JSON(http.StatusOK, gin.H{"member": member})
}

func handleRemoveMember(c *gin.Context, pool *pgxpool.Pool) {
	if !requireManage(c, pool) {
		return
	}
	if !httpapi.ValidID(c.Param("memberId")) {
		httpapi.Error(c, http.StatusNotFound, "NOT_FOUND", "Not found")
		return
	}
	current, err := memberRole(c.Request.Context(), pool, c.Param("teamId"), c.Param("memberId"))
	if httpapi.WriteRead(c, err) {
		return
	}
	actor, _ := access.TeamRole(c.Request.Context(), pool, auth.Current(c).ID, c.Param("teamId"))
	if current == access.RoleOwner && actor != access.RoleOwner {
		httpapi.Error(c, http.StatusForbidden, "FORBIDDEN", "Only an owner can remove an owner")
		return
	}
	err = removeMember(c.Request.Context(), pool, c.Param("teamId"), c.Param("memberId"))
	if errors.Is(err, ErrLastOwner) {
		httpapi.Validation(c, map[string]string{"role": "The team needs an owner"})
		return
	}
	if httpapi.WriteRead(c, err) {
		return
	}
	c.Status(http.StatusNoContent)
}

func requireManage(c *gin.Context, pool *pgxpool.Pool) bool {
	if !httpapi.ValidID(c.Param("teamId")) {
		httpapi.Error(c, http.StatusNotFound, "NOT_FOUND", "Not found")
		return false
	}
	role, err := access.TeamRole(c.Request.Context(), pool, auth.Current(c).ID, c.Param("teamId"))
	if httpapi.WriteRead(c, err) {
		return false
	}
	if !access.CanManage(role) {
		httpapi.Forbidden(c)
		return false
	}
	return true
}
