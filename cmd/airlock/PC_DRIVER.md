# Airlock PC driver protocol

This document describes the Unix-socket protocol currently implemented by
Airlock. The PC driver is the host-side client for the serial socket.

## Socket endpoints

| Purpose | Path | Direction |
| --- | --- | --- |
| PC driver ↔ Airlock host interface | `/tmp/aerolog.sock` | PC driver connects to Airlock |
| Airlock ↔ AI device I/O | `/tmp/ai-io.socket` | Airlock connects to the AI device |
| AI tool calls ↔ Airlock | `/tmp/ai-tool.socket` | AI device connects to Airlock |

The PC driver only needs `/tmp/aerolog.sock`. The other two sockets are
internal to the AI-device side of the system.

## Framing and connection lifecycle

The PC-driver connection is persistent for one AI request. Use a MessagePack
encoder and decoder directly on the Unix stream; each encoder call writes one
MessagePack value and each decoder call reads one value. There is no length
prefix or newline delimiter.

1. Connect to `/tmp/aerolog.sock`.
2. Send one MessagePack `schemas.Message` prompt request.
3. Keep both halves of the socket open.
4. Read messages until Airlock sends `done` or `error`.
5. If Airlock sends `tool-request`, run the requested host tool and send one
   `tool-result` on the same connection, then continue reading.
6. Close the connection after the terminal `done` or `error` message.

Do not half-close the write side after the prompt: Airlock may need to send a
tool request and receive the matching tool result on the same connection.

## PC driver request

The PC driver sends a MessagePack value matching:

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

For a normal prompt, use `Type: "request"` and populate `Prompt` and/or
`String`. Tool definitions may be supplied in `Tools`.

The nested structures are:

```go
type McpToolCall struct {
    Server    string         `msgpack:"server"`
    Tool      string         `msgpack:"tool"`
    Arguments map[string]any `msgpack:"arguments,omitempty"`
}

type McpCallRequest struct {
    Server string         `msgpack:"server"`
    Call   McpToolCall    `msgpack:"call"`
    Params map[string]any `msgpack:"params,omitempty"`
}

type McpCallResult struct {
    Server string `msgpack:"server"`
    Tool   string `msgpack:"tool"`
    Result any    `msgpack:"result,omitempty"`
    Error  string `msgpack:"error,omitempty"`
}
```

## Messages from Airlock

While the AI device produces output, Airlock forwards each valid UTF-8 chunk:

```go
Message{
    Type:   "output",
    String: "<raw AI output>",
}
```

`String` contains raw UTF-8 text. Concatenate `output` messages in their
receive order. A final successful exchange ends with `Message{Type: "done"}`;
the `done` message does not repeat the output. The PC driver must reject or
report any output value that is not valid UTF-8.

If Airlock cannot decode the request or cannot exchange with the AI device,
it returns:

```go
Message{
    Type:   "error",
    String: "<human-readable error>",
}
```

The PC driver should treat any response with `Type: "error"` as a failed
request and surface `String` for diagnostics.

## Host tool requests

When the AI device makes a tool call and the user approves it on the physical
buttons, Airlock sends the PC driver a MessagePack request on the active
serial connection:

```go
Message{
    Type:    "tool-request",
    Request: /* requested McpCallRequest */,
}
```

The display shows the tool name. The right button approves and the left button
denies. On approval, the PC driver must run the requested tool and reply on
the same connection:

```go
Message{
    Type: "tool-result",
    Response: McpCallResult{
        Server: "...",
        Tool:   "...",
        Result: /* MessagePack-compatible result */,
        // Error: "...", // use this instead if the tool fails
    },
}
```

Airlock forwards that result to the AI device through `/tmp/ai-tool.socket`.
If the user denies the request, Airlock sends the AI a generic failed
`tool-result` without contacting the PC driver.

If the tool cannot be approved or the host driver is unavailable, the AI gets
a MessagePack result like:

```go
Message{
    Type: "tool-result",
    Response: McpCallResult{
        Error: "tool request denied by user",
    },
}
```

## Reference client sequence

Pseudo-code for the PC driver:

```text
conn = unix_connect("/tmp/aerolog.sock")
write(conn, msgpack_encode(request))
while true:
    response = msgpack_decode(conn)
    if response.type == "tool-request":
        result = run_host_tool(response.request)
        write(conn, msgpack_encode({type: "tool-result", response: result}))
    else if response.type == "error":
        report(response.string)
        break
    else if response.type == "output":
        consume_utf8_output(response.string)
    else if response.type == "done":
        break
close(conn)
```

