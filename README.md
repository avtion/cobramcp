# CobraMcp

## Overview

CobraMcp 是一个 Go 语言工具库，用于将 Cobra 命令行应用程序转换为 MCP (Minecraft Protocol) 的 Tool 格式。

思路和部分代码来源于 [mcp-cobra](https://github.com/PlusLemon/mcp-cobra).

与 mcp-cobra 不同，CobraMcp 是基于 [ThinkInAIXYZ/go-mcp](https://github.com/ThinkInAIXYZ/go-mcp) 实现。

你可以通过函数生成的 MCP Tool 集成入你的 SSE / Streamable MCP 服务。

# Quick Start

```bash
go get -u github.com/avtion/cobramcp@latest
```

## Example

```go
package main

import (
	"errors"
	"github.com/ThinkInAIXYZ/go-mcp/server"
	"github.com/ThinkInAIXYZ/go-mcp/transport"
	"github.com/avtion/cobramcp"
	"github.com/spf13/cobra"
	"net/http"
	"time"
)

func newExampleCommand() *cobra.Command {
	c := &cobra.Command{Use: "example"}
	cAdd := &cobra.Command{
		Use:   "add",
		Short: "Add two numbers",
	}
	cAdd.Flags().Int("a", 0, "First number")
	cAdd.Flags().Int("b", 0, "Second number")
	cAdd.RunE = func(cmd *cobra.Command, args []string) error {
		a, _ := cmd.Flags().GetInt("a")
		b, _ := cmd.Flags().GetInt("b")
		result := a + b
		cmd.Printf("Result: %v", result)
		return nil
	}
	c.AddCommand(cAdd)
	return c
}

func main() {
	tp, mcpHandler, err := transport.NewStreamableHTTPServerTransportAndHandler()
	if err != nil {
		panic(err)
	}
	s, err := server.NewServer(tp)
	if err != nil {
		panic(err)
	}
	tools, err := cobramcp.GenerateMcpTools(newExampleCommand, cobramcp.Option{})
	if err != nil {
		panic(err)
	}
	for name := range tools.Tools {
		tool := tools.Tools[name]
		// Setup cobra command mcp
		s.RegisterTool(tool.Tool, tool.ToolHandler)
	}

	// this example from https://github.com/ThinkInAIXYZ/go-mcp/blob/main/examples/http_handler/main.go
	router := http.NewServeMux()
	router.HandleFunc("/mcp", mcpHandler.HandleMCP().ServeHTTP)
	httpServer := &http.Server{
		Addr:        ":8080",
		Handler:     router,
		IdleTimeout: time.Minute,
	}
	go func() {
		s.Run()
	}()
	httpServer.ListenAndServe()
}

```

# License

MIT License