# Phase 2 — Teams and projects

**Status:** not started

**Depends on:** Phase 1

**Spec:** `plan.md` sections 6.1–6.4, 16, 17, 39, 54, 70

## Goal

A signed-in user can create and open a project, and cannot open another team's project.

## Tasks

- [ ] Users and session or token authentication
- [ ] Teams, memberships (`owner`, `admin`, `member`, `viewer`), and projects
- [ ] Unique `(team_id, user_id)` and `(team_id, slug)`
- [ ] Authorization on every project-scoped request
- [ ] Team, membership, and project CRUD under `/api/v1`
- [ ] Consistent error envelope
- [ ] Project selector UI, replacing the Phase 2 placeholders

## Acceptance criteria

A logged-in user can create and open a project and cannot access projects belonging to unauthorized teams.
