package spawner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/ijimenezr-ic/orchestrator-agent-v2/mcp-server/internal/status"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type AgentInfo struct {
	PID         int       `json:"pid"`
	TaskID      string    `json:"task_id"`
	AgentType   string    `json:"agent_type"`
	Status      string    `json:"status"`
	StartTime   time.Time `json:"start_time"`
	WorktreePath string   `json:"worktree_path"`
	Model       string    `json:"model"`
}

type Spawner struct {
	mu         sync.Mutex
	agents     map[string]*AgentInfo
	EngramURL  string
	DefaultModel string
	tracker    *status.StatusTracker
}

func NewSpawner(engramURL, defaultModel string) *Spawner {
	return &Spawner{
		agents:       make(map[string]*AgentInfo),
		EngramURL:    engramURL,
		DefaultModel: defaultModel,
		tracker:      &status.StatusTracker{EngramURL: engramURL},
	}
}

func (sp *Spawner) RegisterTools(s *server.MCPServer) {
	spawnTool := mcp.NewTool("agent_spawn",
		mcp.WithDescription("Spawns a subagent as a separate OS process"),
		mcp.WithString("task_id", mcp.Required(), mcp.Description("Unique task identifier")),
		mcp.WithString("agent_type", mcp.Required(), mcp.Description("Type of agent to spawn")),
		mcp.WithString("task_description", mcp.Required(), mcp.Description("Task description for the agent")),
		mcp.WithString("worktree_path", mcp.Required(), mcp.Description("Path to the git worktree")),
		mcp.WithString("model", mcp.Description("Model to use (optional)")),
	)
	s.AddTool(spawnTool, sp.handleSpawn)

	statusTool := mcp.NewTool("agent_status",
		mcp.WithDescription("Gets the status of a running agent"),
		mcp.WithString("task_id", mcp.Required(), mcp.Description("Task ID to get status for")),
	)
	s.AddTool(statusTool, sp.handleStatus)

	listTool := mcp.NewTool("agent_list",
		mcp.WithDescription("Lists all agents with their current status"),
	)
	s.AddTool(listTool, sp.handleList)

	cancelTool := mcp.NewTool("agent_cancel",
		mcp.WithDescription("Cancels an agent by killing its process"),
		mcp.WithString("task_id", mcp.Required(), mcp.Description("Task ID to cancel")),
	)
	s.AddTool(cancelTool, sp.handleCancel)

	resultTool := mcp.NewTool("agent_result",
		mcp.WithDescription("Gets the final result of a completed agent from Engram"),
		mcp.WithString("task_id", mcp.Required(), mcp.Description("Task ID to get result for")),
	)
	s.AddTool(resultTool, sp.handleResult)
}

func (sp *Spawner) handleSpawn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	taskID := req.GetString("task_id", "")
	agentType := req.GetString("agent_type", "")
	taskDescription := req.GetString("task_description", "")
	worktreePath := req.GetString("worktree_path", "")
	model := req.GetString("model", "")
	if model == "" {
		model = sp.DefaultModel
	}

	systemPrompt := fmt.Sprintf("You are a %s agent. Work in the directory: %s", agentType, worktreePath)
	promptArg := fmt.Sprintf("%s\n\n%s", systemPrompt, taskDescription)

	cmd := exec.Command("opencode", "-p", promptArg, "--model", model)
	cmd.Dir = worktreePath
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("ENGRAM_URL=%s", sp.EngramURL),
		fmt.Sprintf("task_id=%s", taskID),
	)

	if err := cmd.Start(); err != nil {
		return errorResult(fmt.Sprintf("failed to start agent: %v", err)), nil
	}

	info := &AgentInfo{
		PID:          cmd.Process.Pid,
		TaskID:       taskID,
		AgentType:    agentType,
		Status:       "running",
		StartTime:    time.Now(),
		WorktreePath: worktreePath,
		Model:        model,
	}

	sp.mu.Lock()
	sp.agents[taskID] = info
	sp.mu.Unlock()

	_ = sp.tracker.ReportStatus("orchestrator", taskID, "running", fmt.Sprintf("agent spawned with PID %d", cmd.Process.Pid))

	go func() {
		err := cmd.Wait()
		sp.mu.Lock()
		if a, ok := sp.agents[taskID]; ok {
			if err != nil {
				a.Status = "failed"
			} else {
				a.Status = "completed"
			}
		}
		sp.mu.Unlock()
		finalStatus := "completed"
		msg := "agent finished successfully"
		if err != nil {
			finalStatus = "failed"
			msg = fmt.Sprintf("agent exited with error: %v", err)
		}
		_ = sp.tracker.ReportStatus("orchestrator", taskID, finalStatus, msg)
	}()

	result := map[string]interface{}{
		"pid":     cmd.Process.Pid,
		"task_id": taskID,
		"status":  "running",
		"model":   model,
	}
	return jsonResult(result), nil
}

func (sp *Spawner) handleStatus(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	taskID := req.GetString("task_id", "")

	sp.mu.Lock()
	info, ok := sp.agents[taskID]
	sp.mu.Unlock()

	engramStatus, _ := sp.tracker.GetStatus(taskID)

	if !ok {
		result := map[string]interface{}{
			"task_id":       taskID,
			"status":        "unknown",
			"engram_status": engramStatus,
		}
		return jsonResult(result), nil
	}

	result := map[string]interface{}{
		"task_id":       info.TaskID,
		"pid":           info.PID,
		"agent_type":    info.AgentType,
		"status":        info.Status,
		"start_time":    info.StartTime,
		"worktree_path": info.WorktreePath,
		"model":         info.Model,
		"engram_status": engramStatus,
	}
	return jsonResult(result), nil
}

func (sp *Spawner) handleList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sp.mu.Lock()
	agents := make([]*AgentInfo, 0, len(sp.agents))
	for _, a := range sp.agents {
		agents = append(agents, a)
	}
	sp.mu.Unlock()

	result := map[string]interface{}{
		"agents": agents,
		"count":  len(agents),
	}
	return jsonResult(result), nil
}

func (sp *Spawner) handleCancel(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	taskID := req.GetString("task_id", "")

	sp.mu.Lock()
	info, ok := sp.agents[taskID]
	sp.mu.Unlock()

	if !ok {
		return errorResult(fmt.Sprintf("no agent found for task_id: %s", taskID)), nil
	}

	proc, err := os.FindProcess(info.PID)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to find process %d: %v", info.PID, err)), nil
	}

	if err := proc.Kill(); err != nil {
		return errorResult(fmt.Sprintf("failed to kill process %d: %v", info.PID, err)), nil
	}

	sp.mu.Lock()
	if a, ok := sp.agents[taskID]; ok {
		a.Status = "cancelled"
	}
	sp.mu.Unlock()

	_ = sp.tracker.ReportStatus("orchestrator", taskID, "cancelled", "agent cancelled by orchestrator")

	result := map[string]string{
		"task_id": taskID,
		"status":  "cancelled",
	}
	return jsonResult(result), nil
}

func (sp *Spawner) handleResult(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	taskID := req.GetString("task_id", "")

	resultData, err := sp.tracker.GetResult(taskID)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to get result from Engram: %v", err)), nil
	}

	result := map[string]string{
		"task_id": taskID,
		"result":  resultData,
	}
	return jsonResult(result), nil
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
