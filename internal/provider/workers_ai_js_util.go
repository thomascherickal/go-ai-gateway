//go:build js && wasm

package provider

import (
	"errors"
	"fmt"
	"syscall/js"

	"github.com/thomas/go-ai-gateway/internal/router"
	"github.com/thomas/go-ai-gateway/internal/types"
)

func extractAIContent(res js.Value) string {
	if respVal := res.Get("response"); !respVal.IsUndefined() && !respVal.IsNull() && respVal.Type() == js.TypeString {
		return respVal.String()
	}
	choices := res.Get("choices")
	if !choices.IsUndefined() && !choices.IsNull() && choices.Length() > 0 {
		msg := choices.Index(0).Get("message")
		if !msg.IsUndefined() && !msg.IsNull() {
			c := msg.Get("content")
			if !c.IsUndefined() && !c.IsNull() {
				return c.String()
			}
		}
	}
	return ""
}

func extractAIUsage(res js.Value, msgs []types.ChatMessage, content string) types.Usage {
	u := res.Get("usage")
	if !u.IsUndefined() && !u.IsNull() {
		pt := maybeJSInt(u.Get("prompt_tokens"))
		ct := maybeJSInt(u.Get("completion_tokens"))
		tt := maybeJSInt(u.Get("total_tokens"))
		if tt == 0 && (pt > 0 || ct > 0) {
			tt = pt + ct
		}
		if tt > 0 {
			return types.Usage{PromptTokens: pt, CompletionTokens: ct, TotalTokens: tt}
		}
	}
	pt := router.EstimateMessagesTokens(msgs)
	ct := router.EstimateTextTokens(content)
	return types.Usage{PromptTokens: pt, CompletionTokens: ct, TotalTokens: pt + ct, Estimated: true}
}

func maybeJSInt(v js.Value) int {
	if v.IsUndefined() || v.IsNull() || v.Type() != js.TypeNumber {
		return 0
	}
	return v.Int()
}

func awaitPromise(p js.Value) (js.Value, error) {
	resCh := make(chan js.Value, 1)
	errCh := make(chan error, 1)
	var thenFn, catchFn js.Func
	thenFn = js.FuncOf(func(_ js.Value, args []js.Value) any {
		defer thenFn.Release()
		if len(args) > 0 {
			resCh <- args[0]
		} else {
			resCh <- js.Undefined()
		}
		return js.Undefined()
	})
	catchFn = js.FuncOf(func(_ js.Value, args []js.Value) any {
		defer catchFn.Release()
		if len(args) > 0 {
			errCh <- fmt.Errorf("workers AI error: %s", args[0].Call("toString").String())
		} else {
			errCh <- errors.New("workers AI promise rejected")
		}
		return js.Undefined()
	})
	p.Call("then", thenFn).Call("catch", catchFn)
	select {
	case v := <-resCh:
		return v, nil
	case err := <-errCh:
		return js.Value{}, err
	}
}
