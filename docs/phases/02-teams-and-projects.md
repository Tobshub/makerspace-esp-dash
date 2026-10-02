# Phase 2 — Teams and projects

**Status:** complete

**Depends on:** Phase 1

**Spec:** `plan.md` sections 6.1–6.4, 16, 17, 39, 54, 70

## Goal

A signed-in user can create and open a project, and cannot open another team's project.

## Tasks

- [x] Users and session or token authentication
- [x] Teams, memberships (`owner`, `admin`, `member`, `viewer`), and projects
- [x] Unique `(team_id, user_id)` and `(team_id, slug)`
- [x] Authorization on every project-scoped request
- [x] Team, membership, and project CRUD under `/api/v1`
- [x] Consistent error envelope
- [x] Project selector UI, replacing the Phase 2 placeholders

## Acceptance criteria

A logged-in user can create and open a project and cannot access projects belonging to unauthorized teams.

## Notes

- Accounts are email and password. The bearer token is random and only its SHA-256 hash is stored. Sessions last 14 days.
- Team slugs are unique across the whole database. Project slugs stay unique per team. Renaming does not change a slug.
- A caller who is not on the owning team gets 404, so the response does not reveal that the team or project exists.
- Viewers are read-only. Members can create and edit projects. Owners and admins manage membership. Only an owner can delete a team or grant the owner role. Only an owner or admin can delete a project.
- Adding a member by email tells the caller when that account does not exist yet.
- The sidebar project selector lists the signed-in user's teams and projects. Overview, metrics, controls, alerts, and events stay placeholders for later phases.
