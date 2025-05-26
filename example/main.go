package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/ThinkInAIXYZ/go-mcp/server"
	"github.com/ThinkInAIXYZ/go-mcp/transport"
	"github.com/avtion/cobramcp"
	"github.com/spf13/cobra"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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
	errCh := make(chan error, 3)
	go func() {
		errCh <- s.Run()
	}()

	go func() {
		if err = httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	if err = signalWaiter(errCh); err != nil {
		panic(fmt.Sprintf("signal waiter: %v", err))
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	httpServer.RegisterOnShutdown(func() {
		if err = s.Shutdown(ctx); err != nil {
			panic(err)
		}
	})

	if err = httpServer.Shutdown(ctx); err != nil {
		panic(err)
	}
}

func signalWaiter(errCh chan error) error {
	signalToNotify := []os.Signal{syscall.SIGINT, syscall.SIGHUP, syscall.SIGTERM}
	if signal.Ignored(syscall.SIGHUP) {
		signalToNotify = []os.Signal{syscall.SIGINT, syscall.SIGTERM}
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, signalToNotify...)
	select {
	case sig := <-signals:
		switch sig {
		case syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM:
			log.Printf("Received signal: %s\n", sig)
			// graceful shutdown
			return nil
		}
	case err := <-errCh:
		return err
	}
	return nil
}
