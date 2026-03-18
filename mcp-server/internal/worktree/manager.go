package worktree

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type Manager struct {
	WorktreeBaseDir string
	RepoDir         string
}

func NewManager(worktreeBaseDir, repoDir string) *Manager {
	return &Manager{
		WorktreeBaseDir: worktreeBaseDir,
		RepoDir:         repoDir,
	}
}

func (m *Manager) RegisterTools(s *server.MCPServer) {
	createTool := mcp.NewTool("worktree_create",
		mcp.WithDescription("Creates a git worktree for an agent task"),
		mcp.WithString("task_id", mcp.Required(), mcp.Description("Unique task identifier")),
		mcp.WithString("branch_name", mcp.Required(), mcp.Description("Branch name to create for the worktree")),
	)
	s.AddTool(createTool, m.handleCreate)

	listTool := mcp.NewTool("worktree_list",
		mcp.WithDescription("Lists all active git worktrees"),
	)
	s.AddTool(listTool, m.handleList)

	removeTool := mcp.NewTool("worktree_remove",
		mcp.WithDescription("Removes a git worktree for a given task"),
		mcp.WithString("task_id", mcp.Required(), mcp.Description("Task ID whose worktree to remove")),
		mcp.WithBoolean("force", mcp.Description("Force removal even if dirty")),
	)
	s.AddTool(removeTool, m.handleRemove)

	mergeTool := mcp.NewTool("worktree_merge",
		mcp.WithDescription("Merges a worktree branch into a target branch"),
		mcp.WithString("source_branch", mcp.Required(), mcp.Description("Branch to merge from")),
		mcp.WithString("target_branch", mcp.Required(), mcp.Description("Branch to merge into")),
	)
	s.AddTool(mergeTool, m.handleMerge)

	cleanupTool := mcp.NewTool("worktree_cleanup",
		mcp.WithDescription("Removes all orchestrator-managed worktrees"),
	)
	s.AddTool(cleanupTool, m.handleCleanup)
}

func (m *Manager) handleCreate(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	taskID := req.GetString("task_id", "")
	branchName := req.GetString("branch_name", "")

	worktreePath := filepath.Join(m.WorktreeBaseDir, taskID)
	if err := os.MkdirAll(m.WorktreeBaseDir, 0755); err != nil {
		return errorResult(fmt.Sprintf("failed to create base dir: %v", err)), nil
	}

	cmd := exec.CommandContext(ctx, "git", "worktree", "add", "-b", branchName, worktreePath)
	cmd.Dir = m.RepoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errorResult(fmt.Sprintf("git worktree add failed: %v\n%s", err, out)), nil
	}

	result := map[string]string{
		"task_id":      taskID,
		"branch":       branchName,
		"path":         worktreePath,
		"status":       "created",
		"git_output":   string(out),
	}
	return jsonResult(result), nil
}

func (m *Manager) handleList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cmd := exec.CommandContext(ctx, "git", "worktree", "list", "--porcelain")
	cmd.Dir = m.RepoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errorResult(fmt.Sprintf("git worktree list failed: %v\n%s", err, out)), nil
	}

	worktrees := parseWorktreePorcelain(string(out))
	result := map[string]interface{}{
		"worktrees": worktrees,
		"count":     len(worktrees),
	}
	return jsonResult(result), nil
}

func (m *Manager) handleRemove(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	taskID := req.GetString("task_id", "")
	force := req.GetBool("force", false)

	worktreePath := filepath.Join(m.WorktreeBaseDir, taskID)

	args := []string{"worktree", "remove", worktreePath}
	if force {
		args = append(args, "--force")
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = m.RepoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errorResult(fmt.Sprintf("git worktree remove failed: %v\n%s", err, out)), nil
	}

	result := map[string]string{
		"task_id": taskID,
		"path":    worktreePath,
		"status":  "removed",
	}
	return jsonResult(result), nil
}

func (m *Manager) handleMerge(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sourceBranch := req.GetString("source_branch", "")
	targetBranch := req.GetString("target_branch", "")

	// Checkout target branch first
	checkoutCmd := exec.CommandContext(ctx, "git", "checkout", targetBranch)
	checkoutCmd.Dir = m.RepoDir
	if out, err := checkoutCmd.CombinedOutput(); err != nil {
		return errorResult(fmt.Sprintf("git checkout failed: %v\n%s", err, out)), nil
	}

	mergeCmd := exec.CommandContext(ctx, "git", "merge", sourceBranch)
	mergeCmd.Dir = m.RepoDir
	out, err := mergeCmd.CombinedOutput()
	if err != nil {
		return errorResult(fmt.Sprintf("git merge failed: %v\n%s", err, out)), nil
	}

	result := map[string]string{
		"source_branch": sourceBranch,
		"target_branch": targetBranch,
		"status":        "merged",
		"git_output":    string(out),
	}
	return jsonResult(result), nil
}

func (m *Manager) handleCleanup(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	entries, err := os.ReadDir(m.WorktreeBaseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return jsonResult(map[string]interface{}{"removed": []string{}, "count": 0}), nil
		}
		return errorResult(fmt.Sprintf("failed to read worktree dir: %v", err)), nil
	}

	var removed []string
	var errs []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		worktreePath := filepath.Join(m.WorktreeBaseDir, entry.Name())
		cmd := exec.CommandContext(ctx, "git", "worktree", "remove", "--force", worktreePath)
		cmd.Dir = m.RepoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v (%s)", entry.Name(), err, strings.TrimSpace(string(out))))
		} else {
			removed = append(removed, entry.Name())
		}
	}

	result := map[string]interface{}{
		"removed": removed,
		"count":   len(removed),
		"errors":  errs,
	}
	return jsonResult(result), nil
}

func parseWorktreePorcelain(output string) []map[string]string {
	var worktrees []map[string]string
	current := map[string]string{}

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if len(current) > 0 {
				worktrees = append(worktrees, current)
				current = map[string]string{}
			}
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 {
			current[parts[0]] = parts[1]
		} else {
			current[line] = "true"
		}
	}
	if len(current) > 0 {
		worktrees = append(worktrees, current)
	}
	return worktrees
}

func jsonResult(v interface{}) *mcp.CallToolResult {
	b, err := json.Marshal(v)
	if err != nil {
		return mcp.NewToolResultText(fmt.Sprintf(`{"error":"failed to marshal result: %v"}`, err))
	}
	return mcp.NewToolResultText(string(b))
}

func errorResult(msg string) *mcp.CallToolResult {
	b, err := json.Marshal(map[string]string{"error": msg})
	if err != nil {
		return mcp.NewToolResultText(`{"error":"internal marshaling error"}`)
	}
	return mcp.NewToolResultText(string(b))
}
