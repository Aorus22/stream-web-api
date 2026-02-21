package script_executor

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	lua "github.com/yuin/gopher-lua"
)

// LuaExecutor handles Lua execution using gopher-lua
type LuaExecutor struct{}

func (lx *LuaExecutor) Run(ctx context.Context, code, url, pageType string) (*ExecuteResult, error) {
	L := lua.NewState()
	defer L.Close()
	L.SetContext(ctx)

	// Globals
	L.SetGlobal("ARG_FULL_URL", lua.LString(url))
	L.SetGlobal("ARG_PAGE_TYPE", lua.LString(pageType))

	// Helper: fetch(url)
	L.SetGlobal("fetch", L.NewFunction(func(L *lua.LState) int {
		reqUrl := L.CheckString(1)
		req, _ := http.NewRequestWithContext(ctx, "GET", reqUrl, nil)
		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LString(string(body)))
		L.Push(lua.LNil)
		return 2
	}))

	// Register GoQuery (Cheerio-like) binding
	lx.registerGoQuery(L)

	if err := L.DoString(code); err != nil {
		return &ExecuteResult{Error: err.Error()}, nil
	}

	ret := L.Get(-1)
	if ret == lua.LNil {
		return &ExecuteResult{Error: "Script returned nil"}, nil
	}

	return &ExecuteResult{Result: lx.luaToInterface(ret)}, nil
}

// registerGoQuery adds CSS selector capabilities to Lua
func (lx *LuaExecutor) registerGoQuery(L *lua.LState) {
	// Function: html_parse(html_string)
	L.SetGlobal("html_parse", L.NewFunction(func(L *lua.LState) int {
		html := L.CheckString(1)
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lx.newSelection(L, doc.Selection))
		return 1
	}))
}

// newSelection wraps a goquery.Selection into a Lua table with methods
func (lx *LuaExecutor) newSelection(L *lua.LState, s *goquery.Selection) *lua.LTable {
	t := L.NewTable()

	// selection:find(selector)
	L.SetField(t, "find", L.NewFunction(func(L *lua.LState) int {
		selector := L.CheckString(2)
		L.Push(lx.newSelection(L, s.Find(selector)))
		return 1
	}))

	// selection:text()
	L.SetField(t, "text", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(strings.TrimSpace(s.Text())))
		return 1
	}))

	// selection:attr(name)
	L.SetField(t, "attr", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(2)
		val, _ := s.Attr(name)
		L.Push(lua.LString(val))
		return 1
	}))

	// selection:first()
	L.SetField(t, "first", L.NewFunction(func(L *lua.LState) int {
		L.Push(lx.newSelection(L, s.First()))
		return 1
	}))

	// selection:last()
	L.SetField(t, "last", L.NewFunction(func(L *lua.LState) int {
		L.Push(lx.newSelection(L, s.Last()))
		return 1
	}))

	// selection:eq(index)
	L.SetField(t, "eq", L.NewFunction(func(L *lua.LState) int {
		index := L.CheckInt(2)
		L.Push(lx.newSelection(L, s.Eq(index)))
		return 1
	}))

	// selection:count()
	L.SetField(t, "count", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LNumber(s.Length()))
		return 1
	}))

	// selection:each(function(index, selection))
	L.SetField(t, "each", L.NewFunction(func(L *lua.LState) int {
		fn := L.CheckFunction(2)
		s.Each(func(i int, sel *goquery.Selection) {
			L.Push(fn)
			L.Push(lua.LNumber(i))
			L.Push(lx.newSelection(L, sel))
			L.PCall(2, 0, nil)
		})
		return 0
	}))

	return t
}

func (lx *LuaExecutor) luaToInterface(v lua.LValue) interface{} {
	switch v.Type() {
	case lua.LTString:
		return string(v.(lua.LString))
	case lua.LTNumber:
		return float64(v.(lua.LNumber))
	case lua.LTBool:
		return bool(v.(lua.LBool))
	case lua.LTTable:
		tbl := v.(*lua.LTable)
		if tbl.MaxN() > 0 { // Array-like
			arr := make([]interface{}, 0, tbl.MaxN())
			for i := 1; i <= tbl.MaxN(); i++ {
				arr = append(arr, lx.luaToInterface(tbl.RawGetInt(i)))
			}
			return arr
		}
		// Map-like
		res := make(map[string]interface{})
		tbl.ForEach(func(key, val lua.LValue) {
			res[key.String()] = lx.luaToInterface(val)
		})
		return res
	default:
		return nil
	}
}
