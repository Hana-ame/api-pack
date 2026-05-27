package shijima_bot

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
)

// Context carries the triggering message info from the forum.
type Context struct {
	Bot       string `json:"bot"`
	Query     string `json:"query"`
	MessageID int    `json:"message_id"`
	ChannelID int    `json:"channel_id"`
	UserID    string `json:"user_id"`
}

// Engine dispatches bot invocations.
type Engine struct {
	builtins map[string]func(ctx Context) (string, error)
}

func NewEngine() *Engine {
	e := &Engine{builtins: make(map[string]func(ctx Context) (string, error))}
	e.registerBuiltins()
	return e
}

func (e *Engine) Register(name string, fn func(ctx Context) (string, error)) {
	if !strings.HasPrefix(name, "@") {
		name = "@" + name
	}
	e.builtins[name] = fn
}

func (e *Engine) Dispatch(ctx Context) (string, error) {
	if !strings.HasPrefix(ctx.Bot, "@") {
		ctx.Bot = "@" + ctx.Bot
	}
	fn, ok := e.builtins[ctx.Bot]
	if !ok {
		return "", fmt.Errorf("unknown bot: %s", ctx.Bot)
	}
	return fn(ctx)
}

func (e *Engine) ListBots() []string {
	names := make([]string, 0, len(e.builtins))
	for name := range e.builtins {
		names = append(names, name)
	}
	return names
}

// ---- Builtin bots ----

func (e *Engine) registerBuiltins() {
	e.Register("@board", builtinBoard)
	e.Register("@rd", builtinDice)
	e.Register("@random", builtinDice)
	e.Register("@reaction", builtinReaction)
}

func builtinBoard(ctx Context) (string, error) {
	parts := strings.SplitN(ctx.Query, " ", 2)
	bid, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
	if bid == 0 {
		return "", fmt.Errorf("invalid board: %s", ctx.Query)
	}
	name := ""
	if len(parts) > 1 {
		name = parts[1]
	}
	if name != "" {
		return fmt.Sprintf(`{"@type":"board","bid":%d,"name":"%s"}`, bid, name), nil
	}
	return fmt.Sprintf(`{"@type":"board","bid":%d}`, bid), nil
}

func builtinDice(ctx Context) (string, error) {
	parts := strings.Split(ctx.Query, "d")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid dice: %s", ctx.Query)
	}
	t, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
	d, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
	if d < 1 {
		d = 1
	}
	if t > 100 {
		return "", fmt.Errorf("too many dice: %d", t)
	}
	rng := rand.New(rand.NewSource(int64(ctx.MessageID)))
	sum := 0
	for i := 0; i < t; i++ {
		sum += rng.Intn(d) + 1
	}
	return ctx.Query + " = " + strconv.Itoa(sum), nil
}

func builtinReaction(ctx Context) (string, error) {
	// Reaction is handled by the callback — bot just acknowledges.
	return "", nil
}

// BuildReplyMessage converts bot response into the forum v3 message body.
func BuildReplyMessage(botName, responseText string, ctx Context) map[string]interface{} {
	if responseText == "" {
		return nil
	}
	// Try to extract simple text from JSON @type wrapper
	var wrapper map[string]interface{}
	if json.Unmarshal([]byte(responseText), &wrapper) == nil {
		if t, ok := wrapper["@type"]; ok {
			switch t {
			case "text":
				if txt, ok := wrapper["text"].(string); ok {
					responseText = txt
				}
			}
		}
	}
	return map[string]interface{}{
		"parent_id": ctx.MessageID,
		"author_id": "bot",
		"name":      botName,
		"content":   responseText,
	}
}
