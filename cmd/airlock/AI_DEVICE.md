# Airlock AI-device protocol

This document describes the AI-side Unix-socket contract currently used by
Airlock.

## Socket endpoints

| Socket | AI-device role | Airlock role |
| --- | --- | --- |
| `/tmp/ai-io.socket` | Listen as the AI I/O server | Connect and send requests |
| `/tmp/ai-tool.socket` | Connect as the AI tool client | Listen and return tool results |

The sockets use Unix stream connections. Each connection carries exactly one
request and one response.

## I/O socket: `/tmp/ai-io.socket`

The AI device must create and listen on `/tmp/ai-io.socket`.

For each connection:

1. Accept the connection.
2. Read one MessagePack-encoded `schemas.Message` until EOF.
3. Process the request.
4. Write the AI response as raw UTF-8 bytes only.
5. Close the connection.

Airlock sends one MessagePack request, then half-closes its write side. The AI
device must not return MessagePack on this socket. The response is an
unframed UTF-8 byte stream, so closing the connection signals that the AI
response is complete.

The AI device must ensure the response is valid UTF-8. Invalid UTF-8 causes
Airlock to reject the exchange.

### Request envelope

```go
type Message struct {
    Type     string         `msgpack:"type"`
    String   string         `msgpack:"string,omitempty"`
    Prompt   string         `msgpack:"prompt,omitempty"`
    Tools    []McpToolCall  `msgpack:"tools,omitempty"`
    Request  McpCallRequest `msgpack:"request,omitempty"`
    Response McpCallResult  `msgpack:"response,omitempty"`
}
```

For a normal prompt, the AI device should use the `Prompt` and/or `String`
fields and set `Type` to `"request"`.

Example request conceptually equivalent to:

```go
schemas.Message{
    Type:   "request",
    Prompt: "Summarize the attached log.",
}
```

The actual wire value is MessagePack, not JSON or Go source text.

### Completion semantics

The AI device may stream output while processing. Airlock validates each UTF-8
chunk and forwards it to the PC driver as it arrives:

```go
schemas.Message{
    Type:   "output",
    String: "<AI output>",
}
```

When the AI device is done, it must finish writing and close the connection.
Airlock then sends `schemas.Message{Type: "done"}` to the PC driver.

## Tool socket: `/tmp/ai-tool.socket`

The AI device connects to `/tmp/ai-tool.socket` when it needs a tool call.

For each tool call:

1. Connect to the socket.
2. Write one MessagePack-encoded `schemas.Message` containing the tool call.
3. Half-close the write side (`shutdown(SHUT_WR)`).
4. Read one MessagePack response until EOF.
5. Decode the response and close the connection.

Do not send multiple tool calls on one connection.

### Tool request

Use `Type: "tool-request"` or another agreed tool-request type and populate
`Request`:

```go
schemas.Message{
    Type: "tool-request",
    Request: schemas.McpCallRequest{
        Server: "example-server",
        Call: schemas.McpToolCall{
            Server: "example-server",
            Tool: "lookup",
            Arguments: map[string]any{
                "query": "example",
            },
        },
    },
}
```

The nested schema is:

```go
type McpCallRequest struct {
    Server string         `msgpack:"server"`
    Call   McpToolCall    `msgpack:"call"`
    Params map[string]any `msgpack:"params,omitempty"`
}

type McpToolCall struct {
    Server    string         `msgpack:"server"`
    Tool      string         `msgpack:"tool"`
    Arguments map[string]any `msgpack:"arguments,omitempty"`
}
```

### Tool response

Tool responses are MessagePack `schemas.Message` values with a nested
`McpCallResult`:

```go
type McpCallResult struct {
    Server string `msgpack:"server"`
    Tool   string `msgpack:"tool"`
    Result any    `msgpack:"result,omitempty"`
    Error  string `msgpack:"error,omitempty"`
}
```

Airlock renders the tool name on its display and waits for a physical user
decision. The right button approves; the left button denies. On approval,
Airlock forwards the request to the active PC driver and returns its
`McpCallResult` through this socket. If the user denies the request or a host
driver is unavailable, Airlock returns a generic failure such as:

```go
schemas.Message{
    Type: "tool-result",
    Response: schemas.McpCallResult{
        Error: "tool request denied by user",
    },
}
```

The AI device should treat a non-empty `Response.Error` as a failed tool call.

## Connection summary

```text
AI device                                  Airlock
----------                                 -------
listen /tmp/ai-io.socket       <---------  connect, send MessagePack request
write raw UTF-8 output         --------->  read until EOF

connect /tmp/ai-tool.socket    --------->  listen
send MessagePack tool request  --------->  decode and handle
read MessagePack tool result   <---------  write response, close
```

