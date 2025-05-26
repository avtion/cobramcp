package cobramcp

import (
	"context"
	"fmt"
	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/ThinkInAIXYZ/go-mcp/server"
	"github.com/spf13/cast"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"strings"
)

const (
	AnnotationToolName            = "mcp.tool.name"
	AnnotationToolDescription     = "mcp.tool.description"
	AnnotationToolTitle           = "mcp.tool.title"
	AnnotationToolReadOnlyHint    = "mcp.tool.readOnlyHint"
	AnnotationToolDestructiveHint = "mcp.tool.destructiveHint"
	AnnotationToolIdempotentHint  = "mcp.tool.idempotentHint"
	AnnotationToolOpenWorldHint   = "mcp.tool.openWorldHint"
)

type McpTool struct {
	Tool        *protocol.Tool
	ToolHandler server.ToolHandlerFunc
}

type McpTools struct {
	Tools map[string]*McpTool
}

type Option struct {
	ToolNameGenerator ToolNameGenerator
	PflagValueParser  PflagValueParser
}

type ToolNameGenerator func(cmd *cobra.Command, fullCommandPath []string) string

func GenerateToolName(_ *cobra.Command, fullCommandPath []string) string {
	return strings.Join(fullCommandPath, "_")
}

var _defaultToolNameGenerator ToolNameGenerator = GenerateToolName

type PflagValueParser func(value pflag.Value) (dt protocol.DataType, itemProperty *protocol.Property)

func DefaultPflagValueDateTypeParser(value pflag.Value) (dt protocol.DataType, itemProperty *protocol.Property) {
	switch value.Type() {
	case "string":
		dt = protocol.String
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64":
		dt = protocol.Integer
	case "bool":
		dt = protocol.Boolean
	case "float32", "float64":
		dt = protocol.Number
	case "duration":
		dt = protocol.String // Duration is represented as a string in MCP
	case "int32Slice", "int64Slice":
		dt = protocol.Array
		itemProperty = &protocol.Property{Type: protocol.Integer}
	case "stringSlice":
		dt = protocol.Array
		itemProperty = &protocol.Property{Type: protocol.String}
	default:
		dt = protocol.String // Default to string for unsupported types
	}
	return dt, itemProperty
}

func GenerateMcpTools(commandBuilder func() *cobra.Command, opt Option) (*McpTools, error) {
	m := &McpTools{Tools: make(map[string]*McpTool)}
	c := commandBuilder()
	leaves := GetLeafCommands(c)

	for idx := range leaves {
		leaf := leaves[idx]
		if leaf.Hidden {
			// Skip hidden commands
			continue
		}
		fullPath := GetFullCommandPath(leaf)
		toolName := _defaultToolNameGenerator(leaf, fullPath)
		if opt.ToolNameGenerator != nil {
			toolName = opt.ToolNameGenerator(leaf, fullPath)
		}
		// 支持通过注解设置工具名称和描述，但不推荐修改
		if name, isExist := leaf.Annotations[AnnotationToolName]; isExist && name != "" {
			toolName = name
		}
		toolDesc := leaf.Short
		if toolDesc == "" {
			toolDesc = leaf.Long
		}
		if desc, isExist := leaf.Annotations[AnnotationToolDescription]; isExist && desc != "" {
			toolDesc = desc
		}

		flags := GetAllFlags(leaf)
		inputSchema := &protocol.InputSchema{
			Type:       protocol.Object,
			Properties: make(map[string]*protocol.Property, len(flags)),
			Required:   make([]string, 0, len(flags)),
		}
		for _, f := range flags {
			// Skip hidden flags
			if f.Hidden {
				continue
			}
			// 支持通过注解设置工具名称和描述，但不推荐修改
			flagName := f.Name
			if val, isExist := f.Annotations[AnnotationToolName]; isExist && len(val) > 0 && val[0] != "" {
				flagName = val[0]
			}
			flagDescription := f.Usage
			if f.DefValue == "" {
				inputSchema.Required = append(inputSchema.Required, flagName)
			} else {
				flagDescription = fmt.Sprintf("%s (default: %s)", flagDescription, f.DefValue)
			}
			if val, isExist := f.Annotations[AnnotationToolDescription]; isExist && len(val) > 0 && val[0] != "" {
				flagDescription = val[0]
			}

			dt, itemProperty := DefaultPflagValueDateTypeParser(f.Value)
			if opt.PflagValueParser != nil {
				dt, itemProperty = opt.PflagValueParser(f.Value)
			}
			property := &protocol.Property{
				Type:        dt,
				Description: flagDescription,
				Items:       itemProperty,
			}
			inputSchema.Properties[flagName] = property
		}

		var toolHandlerFunc server.ToolHandlerFunc = func(ctx context.Context, request *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
			args := fullPath
			for argName, argVal := range request.Arguments {
				args = append(args, fmt.Sprintf("--%s=%v", argName, argVal))
			}
			rootCommand := commandBuilder()
			rootCommand.SetArgs(args)
			outStd := &strings.Builder{}
			rootCommand.SetOut(outStd)
			outErr := &strings.Builder{}
			rootCommand.SetErr(outErr)
			resp := &protocol.CallToolResult{
				Content: make([]protocol.Content, 0),
				IsError: false,
			}
			err := rootCommand.ExecuteContext(WithCommandExecContext(ctx, resp))
			// 如果没有输出内容，尝试从标准输出和错误输出中获取内容
			if len(resp.Content) == 0 {
				text := outStd.String()
				if outErr.Len() > 0 {
					text = outErr.String()
					resp.IsError = true
				}
				resp.Content = append(resp.Content, &protocol.TextContent{
					Annotated: protocol.Annotated{},
					Type:      "text",
					Text:      text,
				})
				resp.IsError = err != nil || outErr.Len() > 0
			}
			return resp, err
		}

		title := toolName
		if _title, isExist := leaf.Annotations[AnnotationToolTitle]; isExist && _title != "" {
			title = _title
		}
		var readOnlyHint *bool
		if _readOnlyHint, isExist := leaf.Annotations[AnnotationToolReadOnlyHint]; isExist && _readOnlyHint != "" {
			b := cast.ToBool(_readOnlyHint)
			readOnlyHint = &b
		}
		var idempotentHint *bool
		if _idempotentHint, isExist := leaf.Annotations[AnnotationToolIdempotentHint]; isExist && _idempotentHint != "" {
			b := cast.ToBool(_idempotentHint)
			idempotentHint = &b
		}
		var destructiveHint *bool
		if _destructiveHint, isExist := leaf.Annotations[AnnotationToolDestructiveHint]; isExist && _destructiveHint != "" {
			b := cast.ToBool(_destructiveHint)
			destructiveHint = &b
		}
		var openWorldHint *bool
		if _openWorldHint, isExist := leaf.Annotations[AnnotationToolOpenWorldHint]; isExist && _openWorldHint != "" {
			b := cast.ToBool(_openWorldHint)
			openWorldHint = &b
		}
		toolAnnotations := &protocol.ToolAnnotations{
			Title:           title,
			ReadOnlyHint:    readOnlyHint,
			DestructiveHint: destructiveHint,
			IdempotentHint:  idempotentHint,
			OpenWorldHint:   openWorldHint,
		}
		tool := &McpTool{
			Tool: &protocol.Tool{
				Name:        toolName,
				Description: toolDesc,
				InputSchema: *inputSchema,
				Annotations: toolAnnotations,
			},
			ToolHandler: toolHandlerFunc,
		}
		m.Tools[toolName] = tool
		return m, nil
	}
	return m, nil
}

type _ctxCommandExecKey struct{}

var ctxCommandExecKey = _ctxCommandExecKey{}

func WithCommandExecContext(ctx context.Context, resp *protocol.CallToolResult) context.Context {
	return context.WithValue(ctx, ctxCommandExecKey, resp)
}

func GetCommandFromContext(ctx context.Context) *protocol.CallToolResult {
	if v, ok := ctx.Value(ctxCommandExecKey).(*protocol.CallToolResult); ok {
		return v
	}
	return nil
}

func GetLeafCommands(cmd *cobra.Command) []*cobra.Command {
	var leaves = make([]*cobra.Command, 0)
	if len(cmd.Commands()) == 0 {
		leaves = append(leaves, cmd)
	} else {
		for _, sub := range cmd.Commands() {
			leaves = append(leaves, GetLeafCommands(sub)...)
		}
	}
	return leaves
}

func GetFullCommandPath(cmd *cobra.Command) []string {
	if cmd.Parent() == nil {
		// ignore the root command
		return []string{}
	}
	parentPath := GetFullCommandPath(cmd.Parent())
	return append(parentPath, cmd.Name())
}

func GetAllFlags(cmd *cobra.Command) map[string]*pflag.Flag {
	flags := make(map[string]*pflag.Flag)
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) { flags[f.Name] = f })
	cmd.InheritedFlags().VisitAll(func(f *pflag.Flag) { flags[f.Name] = f })
	return flags
}
