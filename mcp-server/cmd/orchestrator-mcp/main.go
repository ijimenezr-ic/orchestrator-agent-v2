package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/ijimenezr-ic/orchestrator-agent-v2/mcp-server/internal/spawner"
	"github.com/ijimenezr-ic/orchestrator-agent-v2/mcp-server/internal/worktree"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type Config struct {
	MaxSubagents    int
	WorktreeBaseDir string
	EngramURL       string
	GithubToken     string
	SubagentModel   string
	RepoDir         string
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	repoDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get cwd: %v\n", err)
		os.Exit(1)
	}

	maxSubagents := 0
	if v := os.Getenv("MAX_SUBAGENTS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			maxSubagents = n
		}
	}

	cfg := Config{
		MaxSubagents:    maxSubagents,
		WorktreeBaseDir: envOrDefault("WORKTREE_BASE_DIR", "../.worktrees"),
		EngramURL:       envOrDefault("ENGRAM_URL", "http://localhost:7437"),
		GithubToken:     os.Getenv("GITHUB_TOKEN"),
		SubagentModel:   envOrDefault("SUBAGENT_MODEL", "claude-sonnet-4-5-20250514"),
		RepoDir:         repoDir,
	}

	s := server.NewMCPServer(
		"orchestrator-mcp",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	// Register worktree tools
	wm := worktree.NewManager(cfg.WorktreeBaseDir, cfg.RepoDir)
	wm.RegisterTools(s)

	// Register spawner tools
	sp := spawner.NewSpawner(cfg.EngramURL, cfg.SubagentModel)
	sp.RegisterTools(s)

	// Register a simple info tool showing config (excluding secrets)
	infoTool := mcp.NewTool("orchestrator_info",
		mcp.WithDescription("Returns the current orchestrator configuration"),
	)
	s.AddTool(infoTool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		info := fmt.Sprintf(`{"max_subagents":%d,"worktree_base_dir":%q,"engram_url":%q,"subagent_model":%q}`,
			cfg.MaxSubagents, cfg.WorktreeBaseDir, cfg.EngramURL, cfg.SubagentModel)
		return mcp.NewToolResultText(info), nil
	})

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
