package main

import (
	"context"
	"github.com/ThinkInAIXYZ/go-mcp/client"
	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/ThinkInAIXYZ/go-mcp/transport"
	"testing"
)

func TestListTools(t *testing.T) {
	tp, err := transport.NewStreamableHTTPClientTransport("http://localhost:8080/mcp")
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}
	c, err := client.NewClient(tp)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	resListTools, err := c.ListTools(context.TODO())
	if err != nil {
		t.Fatalf("failed to list tools: %v", err)
	}
	for _, v := range resListTools.Tools {
		t.Logf("Tool Name: %s, Description: %s", v.Name, v.Description)
	}
}

func TestAdd(t *testing.T) {
	tp, err := transport.NewStreamableHTTPClientTransport("http://localhost:8080/mcp")
	if err != nil {
		t.Fatalf("failed to create transport: %v", err)
	}
	c, err := client.NewClient(tp)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	res, err := c.CallTool(context.TODO(), &protocol.CallToolRequest{
		Name: "add",
		Arguments: map[string]interface{}{
			"a": 5,
			"b": 10,
		},
	})
	if err != nil {
		t.Fatalf("failed to call tool: %v", err)
	}
	if len(res.Content) == 0 {
		t.Fatal("expected result, got empty response")
	}
	if contentType := res.Content[0].GetType(); contentType != "text" {
		t.Fatalf("expected text content, got %s", contentType)
	}
	text := res.Content[0].(*protocol.TextContent).Text
	if text != "Result: 15" {
		t.Fatalf("expected 'Result: 15', got '%s'", text)
	}
	t.Logf("Tool call successful, result: %s", text)
}
