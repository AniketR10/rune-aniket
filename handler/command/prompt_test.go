package command

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/iterator"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	textapi "unstable.build/go-tui/api/text"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
	testutil "unstable.build/go-tui/util/test"
)

func TestCommandHandlerManualsDrawTooSmallForManual(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ShowManualAfter = 0
	cfg.HistoryKey = term.KeyComb{Ch: '@'}
	cfg.FrameCharSet = component.FrameCharSetDefault()

	tsuite := []struct {
		desc         string
		sequence     string
		commands     []text.CommandManual
		expectedDraw string
	}{
		{"initializes no commands empty", "", nil, `
▐                   
                    
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"initializes no commands empty search yields 0", "a", nil, `
a▐                  
                    
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"initializes with some commands", "", goodTestCommands,
			`
▐                   
subaru              
jeep                
mercedes            
gladiator           
current             
                    
                    
                    
                    `},
		{"initializes with some commands search match", "e", goodTestCommands,
			`
e▐                  
jeep                
mercedes            
current             
                    
                    
                    
                    
                    
                    `},
		{"initializes with lots of commands", "", goodLotsTestCommands,
			`
▐                   
0                   
1                   
2                   
3                   
4                   
5                   
6                   
7                   
8                   `},
		{"draw command NOT in list with no args",
			"1", nil, `
1▐                  
                    
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"draw fully typed command with args with auto-complete with expanded last arg and delete in the middle",
			"merce ~^^^^^^^^^erce my", goodTestCommands, `
mercedes my▐        
                    
                    
                    
                    
                    
                    
                    
                    
                    `},
	}

	log.SetLevel(log.InfoLevel)
	for _, tcase := range tsuite {
		tcase := tcase
		t.Run(tcase.desc, func(t *testing.T) {
			t.Parallel()
			dispatchFn, cleanup := nopDispatch()
			defer cleanup(t)

			completeFn, cleanupComplete := nopComplete()
			defer cleanupComplete(t)

			storage := document.NewInMemoryService()
			b := NewPrompt(
				storage, FuncCompleter(completeFn), FuncDispatcher(dispatchFn),
				term.NopInterrupter(), tcase.commands, cfg,
			)
			b.sync = true
			defer b.Close()
			cases := []testutil.HandlerSequenceTestCase{
				{InputSequence: "_" + tcase.sequence + "_", Expected: tcase.expectedDraw[1:]},
			}
			testutil.TestHandlerSequence(t, testCommandHandler{b}, 20, 10, cases)
		})
	}
}

func TestCommandHandlerDispatch(t *testing.T) {
	storage := document.NewInMemoryService()
	cfg := DefaultConfig()
	cfg.ShowManualAfter = 1 * time.Hour
	cfg.HistoryKey = term.KeyComb{Ch: '@'}

	tsuite := []struct {
		desc        string
		sequence    string
		commands    []string
		completeCmd func() (func(ctx context.Context, command string, args ...string) (iterator.Iterator[string], string), func(*testing.T))
		dispatchCmd func() (func(command string, args ...string) bool, func(*testing.T))
	}{
		{"dispatches command NOT in list with no args",
			"1>", []string{"lane", "lorelai", "rori"},
			nopComplete, expectDispatch("1")},
		{"dispatches command in list with no args",
			"lo>", []string{"lane", "lorelai", "rori"},
			nopComplete, expectDispatch("lorelai")},
		{"dispatches command NOT in list with args no auto-complete",
			"1 /tmp/a>", []string{"lane", "lorelai", "rori"},
			nopComplete, expectDispatch("1", "/tmp/a")},
		{"dispatches command with args no auto-complete",
			"lo /tmp/a>", []string{"lane", "lorelai", "rori"},
			nopComplete, expectDispatch("lorelai", "/tmp/a")},
		{"dispatches command with args with auto-complete",
			"ro my#>", []string{"lane", "lorelai", "rori"},
			completeWith("myArg"), expectDispatch("rori", "myArg")},
		{"dispatches fully typed command with args with auto-complete",
			"rori my#>", []string{"lane", "lorelai", "rori"},
			completeWith("myArg"), expectDispatch("rori", "myArg")},
		{"dispatches fully typed command with args with auto-complete and delete in the middle",
			"rori ^ my#>", []string{"lane", "lorelai", "rori"},
			completeWith("myArg"), expectDispatch("rori", "myArg")},
		{"dispatches command with args with auto-complete one last space",
			"ro my# >", []string{"lane", "lorelai", "rori"},
			completeRespectively([]string{"myArg", "myArg"}), expectDispatch("rori", "myArg")},
		{"dispatches command with args with auto-complete space that's removed",
			"ro my# ^>", []string{"lane", "lorelai", "rori"},
			completeRespectively([]string{"myArg", "myArg"}), expectDispatch("rori", "myArg")},
		{"dispatches command with args with auto-complete delete and re-typed all",
			"ro my# ^^^^^^^^^^^^ro my# a>", []string{"lane", "lorelai", "rori"},
			completeWith("myArg"), expectDispatch("rori", "myArg", "a")},
		{"dispatches command from history no autocomplete",
			"rori myArg>lorelai myArg>@ oArg>", []string{"lane", "lorelai", "rori"},
			nopComplete, expectDispatch("lorelai", "myArg", "oArg")},
		{"dispatches command with extra spaces in args no auto-complete",
			"lo   /tmp/a>", []string{"lane", "lorelai", "rori"},
			nopComplete, expectDispatch("lorelai", "/tmp/a")},
		{"dispatches command with extra spaces in args that are deleted no auto-complete",
			"lo   ^^/tmp/a>", []string{"lane", "lorelai", "rori"},
			nopComplete, expectDispatch("lorelai", "/tmp/a")},
		{"dispatches command with multiple args and completer gets called for every character",
			"ro my# oro#>", []string{"lane", "lorelai", "rori"},
			expectCompleteWith(
				[][]string{
					{}, {"m"}, {"my"}, {"myArg"}, {"myArg"},
					{"myArg", "o"}, {"myArg", "or"}, {"myArg", "oro"}, {"myArg", "oregano"},
				},
				[][]string{
					{"myArg"}, {"myArg"}, {"myArg"}, {"oregano", "oregani"}, {"oregano", "oregani"},
					{"oregano", "oregani"}, {"oregano", "oregani"}, {"oregano", "oregani"}, {},
				}),
			expectDispatch("rori", "myArg", "oregano")},
		{"dispatch delete and re-type all with no autocomplete",
			"rori myArg ^^^^^^^^^^^rori myArg a>", []string{"lane", "lorelai", "rori"},
			nopComplete, expectDispatch("rori", "myArg", "a")},
		{"dispatch from history with autocomplete",
			"lo my#>ro my#>@ oArg>", []string{"lane", "lorelai", "rori"},
			completeWith("myArg"), expectDispatch("rori", "myArg", "oArg")},
		{"dispatch delete after load from history with autocomplete",
			"lo my#>ro my#>@^^^^^oArg>", []string{"lane", "lorelai", "rori"},
			completeWith("myArg"), expectDispatch("rori", "oArg")},
		{"dispatch delete after load from history with autocomplete scroll through history",
			"lo my#>ro my#>@@^^^^^oArg>", []string{"lane", "lorelai", "rori"},
			completeWith("myArg"), expectDispatch("lorelai", "oArg")},
		{"dispatch literal arg if history has option but user IS NOT scrolling",
			"lo my#>ro my#>lo m>", []string{"lane", "lorelai", "rori"},
			completeWith("myArg"), expectDispatch("lorelai", "m")},
		{"dispatch complete arg if history has option but user IS scrolling",
			"lo my#>ro my#>lo m*>", []string{"lane", "lorelai", "rori"},
			completeWith("myArg"), expectDispatch("lorelai", "myArg")},
		{"dispatch auto-complete with enter regardless of whether user IS scrolling",
			"lo>", []string{"lane", "lorelai", "rori"},
			completeWith("myArg"), expectDispatch("lorelai")},
		{"dispatch auto-complete with tab and enter regardless of whether user IS scrolling",
			"lo#>", []string{"lane", "lorelai", "rori"},
			completeWith("myArg"), expectDispatch("lorelai")},
	}

	for _, tcase := range tsuite {
		log.SetLevel(log.InfoLevel)
		tcase := tcase
		t.Run(tcase.desc, func(t *testing.T) {
			dispatchFn, cleanup := tcase.dispatchCmd()
			defer cleanup(t)

			completeFn, cleanupComplete := tcase.completeCmd()
			defer cleanupComplete(t)

			interrupter := term.NopInterrupter()
			b := NewPrompt(
				storage, FuncCompleter(completeFn), FuncDispatcher(dispatchFn),
				interrupter, testNoManualCommands(tcase.commands), cfg,
			)
			b.sync = true
			defer b.Close()
			for _, ch := range tcase.sequence {
				b.Wait()
				switch ch {
				case '#':
					b.Handle(term.Event{Type: term.EventKey, Key: term.KeyTab})
				case '*':
					b.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowDown})
				case '>':
					b.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
				case '^':
					b.Handle(term.Event{Type: term.EventKey, Key: term.KeyBackspace})
				default:
					b.Handle(term.Event{Type: term.EventKey, Ch: ch})
				}
			}
		})
	}
}

func TestCommandHandlerDraw(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ShowManualAfter = 1 * time.Hour
	cfg.HistoryKey = term.KeyComb{Ch: '@'}

	tsuite := []struct {
		desc         string
		sequence     string
		commands     []string
		completeCmd  func() (func(ctx context.Context, command string, args ...string) (iterator.Iterator[string], string), func(*testing.T))
		dispatchCmd  func() (func(command string, args ...string) bool, func(*testing.T))
		expectedDraw string
	}{
		{"initializes no commands empty", "", nil, nopComplete, nopDispatch, `
▐                   
                    
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"initializes no commands empty search yields 0", "a", nil, nopComplete, nopDispatch, `
a▐                  
                    
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"initializes with some commands",
			"", []string{"lane", "lorelai", "rori"},
			nopComplete, nopDispatch, `
▐                   
lane                
lorelai             
rori                
                    
                    
                    
                    
                    
                    `},
		{"initializes with some commands search match",
			"l", []string{"lane", "lorelai", "rori"},
			nopComplete, nopDispatch, `
l▐                  
lane                
lorelai             
                    
                    
                    
                    
                    
                    
                    `},
		{"initializes with lots of commands", "", lotsOfCommands,
			nopComplete, nopDispatch, `
▐                   
0                   
1                   
2                   
3                   
4                   
5                   
6                   
7                   
8                   `},
		{"draw command NOT in list with no args",
			"1", []string{"lane", "lorelai", "rori"},
			nopComplete, nopDispatch, `
1▐                  
                    
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"draw command in list with no args",
			"lo", []string{"lane", "lorelai", "rori"},
			nopComplete, nopDispatch, `
lo▐                 
lorelai             
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"draw command NOT in list with args no auto-complete",
			"1 /tmp/a", []string{"lane", "lorelai", "rori"},
			nopComplete, nopDispatch, `
1 /tmp/a▐           
                    
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"draw command in list with args no auto-complete",
			"lo /tmp/a", []string{"lane", "lorelai", "rori"},
			nopComplete, nopDispatch, `
lorelai /tmp/a▐     
                    
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"draw command with args with auto-complete",
			"rori my", []string{"lane", "lorelai", "rori"},
			completeWith("myArg"), nopDispatch, `
rori my▐            
myArg               
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"draw command with args with auto-complete, expand last arg",
			"rori ~my", []string{"lane", "lorelai", "rori"},
			completeWith("expanded/myArg"), nopDispatch, `
rori expanded/my▐   
expanded/myArg      
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"draw fully typed command with args with auto-complete",
			"rori my", []string{"lane", "lorelai", "rori"},
			completeWith("myArg"), nopDispatch, `
rori my▐            
myArg               
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"draw fully typed command with args with auto-complete with expanded last arg and delete in the middle",
			"rori ~^^^^^^^^^my", []string{"lane", "lorelai", "rori"},
			completeWith("myArg"), nopDispatch, `
rori my▐            
myArg               
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"draw fully typed command with args with auto-complete and delete in the middle",
			"rori ^ my", []string{"lane", "lorelai", "rori"},
			completeWith("myArg"), nopDispatch, `
rori my▐            
myArg               
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"draw command with args with auto-complete one last space",
			"ro my ", []string{"lane", "lorelai", "rori"},
			completeRespectively([]string{"myArg"}), nopDispatch, `
rori my ▐           
                    
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"draw command with args with auto-complete one last space that's removed",
			"ro my ^", []string{"lane", "lorelai", "rori"},
			completeRespectively([]string{"myArg"}), nopDispatch, `
rori my▐            
myArg               
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"draw command with args with auto-complete delete and re-typed all",
			"ro my ^^^^^^^^^^^ro my a", []string{"lane", "lorelai", "rori"},
			completeRespectively([]string{"myArg"}), nopDispatch, `
rori my a▐          
                    
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"draw command with args with auto-complete delete and re-typed all with expanded last arg",
			"ro ~my ^^^^^^^^^^^^^^^^^^^^ro my", []string{"lane", "lorelai", "rori"},
			completeWith("myArg"), nopDispatch, `
rori my▐            
myArg               
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"draw delete and re-type all with no autocomplete",
			"rori myArg ^^^^^^^^^^^rori myArg a", []string{"lane", "lorelai", "rori"},
			nopComplete, nopDispatch, `
rori myArg a▐       
                    
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"draw command from history no autocomplete",
			"rori myArg>lorelai myArg>@ oArg", []string{"lane", "lorelai", "rori"},
			nopComplete, expectDispatch("lorelai", "myArg"), `
lorelai myArg oAr   
g▐                  
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"draw from history with autocomplete",
			"lo my✌>ro my✌>@", []string{"lane", "lorelai", "rori"},
			completeWith("myArg"), expectDispatch("rori", "myArg"), `
rori myArg▐         
myArg               
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"draw from history with autocomplete with expanded last arg",
			"lo my✌>ro ~my✌>@", []string{"lane", "lorelai", "rori"},
			completeWith("expanded/myArg"), expectDispatch("rori", "expanded/myArg"), `
rori expanded/myA   
rg▐                 
expanded/myArg      
                    
                    
                    
                    
                    
                    
                    `},
		{"draw delete after load from history with autocomplete",
			"lo my✌>ro my✌>@^^^^^", []string{"lane", "lorelai", "rori"},
			completeWith("myArg"), expectDispatch("rori", "myArg"), `
rori ▐              
myArg               
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"draw delete after load from history with autocomplete scroll through history",
			"lo my✌>ro my✌>@@^^^^^oArg", []string{"lane", "lorelai", "rori"},
			completeWith("myArg"), expectDispatch("rori", "myArg"), `
lorelai oArg▐       
                    
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"draw command in list with extra spaces in args no auto-complete",
			"lo   /tmp/a", []string{"lane", "lorelai", "rori"},
			nopComplete, nopDispatch, `
lorelai   /tmp/a▐   
                    
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"draw command in list with extra spaces in args that are deleted no auto-complete",
			"lo   ^^^ /tmp/a", []string{"lane", "lorelai", "rori"},
			nopComplete, nopDispatch, `
lorelai /tmp/a▐     
                    
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"draw command with multiple args with auto-complete",
			"ro my✌ oro", []string{"lane", "lorelai", "rori"},
			expectCompleteWith(
				[][]string{
					{}, {"m"}, {"my"}, {"myArg"}, {"myArg"},
					{"myArg", "o"}, {"myArg", "or"}, {"myArg", "oro"},
				},
				[][]string{
					{"myArg"}, {"myArg"}, {"myArg"}, {"oregano", "oregani"}, {"oregano", "oregani"},
					{"oregano", "oregani"}, {"oregano", "oregani"}, {"oregano", "oregani"},
				}),
			nopDispatch, `
rori myArg  oro▐    
oregano             
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"no completion uses historical positional args as completion list items",
			"ro myArg>ro my✌", []string{"lane", "lorelai", "rori"},
			expectCompleteWith(
				[][]string{
					{}, {"m"}, {"my"}, {"myA"}, {"myAr"}, {"myArg"},
					{}, {"m"}, {"my"}, {"myArg"},
				},
				[][]string{}),
			expectDispatch("rori", "myArg"), `
rori myArg ▐        
                    
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"no completion uses historical positional args as completion list items (3rd argument)",
			"ro myArg oro>ro myArg or", []string{"lane", "lorelai", "rori"},
			expectCompleteWith(
				[][]string{
					{}, {"m"}, {"my"}, {"myA"}, {"myAr"}, {"myArg"}, {"myArg"}, {"myArg", "o"}, {"myArg", "or"}, {"myArg", "oro"},
					{}, {"m"}, {"my"}, {"myA"}, {"myAr"}, {"myArg"}, {"myArg"}, {"myArg", "o"}, {"myArg", "or"},
				},
				[][]string{}),
			expectDispatch("rori", "myArg", "oro"), `
rori myArg or▐      
oro                 
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"no completion uses historical positional args as completion list items (3rd argument, tab)",
			"ro myArg oro>ro myArg o✌", []string{"lane", "lorelai", "rori"},
			expectCompleteWith(
				[][]string{
					{}, {"m"}, {"my"}, {"myA"}, {"myAr"}, {"myArg"}, {"myArg"}, {"myArg", "o"}, {"myArg", "or"}, {"myArg", "oro"},
					{}, {"m"}, {"my"}, {"myA"}, {"myAr"}, {"myArg"}, {"myArg"}, {"myArg", "o"}, {"myArg", "oro"},
				},
				[][]string{}),
			expectDispatch("rori", "myArg", "oro"), `
rori myArg oro ▐    
                    
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"fuzzy complete tab expands from historical args",
			"ro myArg oro>ro mo✌", []string{"lane", "lorelai", "rori"},
			expectCompleteWith(
				[][]string{
					{}, {"m"}, {"my"}, {"myA"}, {"myAr"}, {"myArg"}, {"myArg"}, {"myArg", "o"}, {"myArg", "or"}, {"myArg", "oro"},
					{}, {"m"}, {"mo"}, {"myArg", "oro"},
				},
				[][]string{}),
			expectDispatch("rori", "myArg", "oro"), `
rori myArg oro ▐    
                    
                    
                    
                    
                    
                    
                    
                    
                    `},
	}

	log.SetLevel(log.InfoLevel)
	for _, tcase := range tsuite {
		tcase := tcase
		t.Run(tcase.desc, func(t *testing.T) {
			dispatchFn, cleanup := tcase.dispatchCmd()
			defer cleanup(t)

			completeFn, cleanupComplete := tcase.completeCmd()
			defer cleanupComplete(t)

			storage := document.NewInMemoryService()
			b := NewPrompt(
				storage, FuncCompleter(completeFn), FuncDispatcher(dispatchFn),
				term.NopInterrupter(), testNoManualCommands(tcase.commands), cfg,
			)
			b.sync = true
			defer b.Close()
			cases := []testutil.HandlerSequenceTestCase{
				{InputSequence: tcase.sequence, Expected: tcase.expectedDraw[1:]},
			}
			testutil.TestHandlerSequence(t, testCommandHandler{b}, 20, 10, cases)
		})
	}
}

type testCommandHandler struct {
	*Prompt
}

func (t testCommandHandler) Handle(ev term.Event) (bool, bool) {
	t.Wait()
	quit, handled := t.Prompt.Handle(ev)
	t.Wait()
	return quit, handled
}

var (
	lotsOfCommands []string
)

func init() {
	for i := 0; i < 100; i++ {
		lotsOfCommands = append(lotsOfCommands, strconv.Itoa(i))
	}
}

func nopComplete() (func(context.Context, string, ...string) (iterator.Iterator[string], string), func(*testing.T)) {
	return func(ctx context.Context, command string, args ...string) (iterator.Iterator[string], string) {
		return iterator.FromSlice[string](nil), ""
	}, func(*testing.T) {}
}

func completeWith(data ...string) func() (func(context.Context, string, ...string) (iterator.Iterator[string], string), func(*testing.T)) {
	return func() (func(ctx context.Context, command string, args ...string) (iterator.Iterator[string], string), func(*testing.T)) {
		return func(ctx context.Context, command string, args ...string) (iterator.Iterator[string], string) {
			if len(args) != 0 && args[len(args)-1] == "~" {
				return iterator.FromSlice(data), "expanded/"
			}
			return iterator.FromSlice(data), ""
		}, func(*testing.T) {}
	}
}

func completeRespectively(data []string) func() (func(context.Context, string, ...string) (iterator.Iterator[string], string), func(*testing.T)) {
	return func() (func(context.Context, string, ...string) (iterator.Iterator[string], string), func(*testing.T)) {
		return func(ctx context.Context, command string, args ...string) (iterator.Iterator[string], string) {
			if len(args) >= len(data) {
				return iterator.FromSlice[string](nil), ""
			}
			completing := []string{data[len(args)]}
			return iterator.FromSlice(completing), ""
		}, func(*testing.T) {}
	}
}

func expectCompleteWith(expectedArgs [][]string, data [][]string) func() (func(ctx context.Context, command string, args ...string) (iterator.Iterator[string], string), func(*testing.T)) {
	var actualArgsSlice [][]string
	var called int
	return func() (func(context.Context, string, ...string) (iterator.Iterator[string], string), func(*testing.T)) {
		return func(ctx context.Context, command string, args ...string) (iterator.Iterator[string], string) {
				if called >= len(data) {
					called++ // cleanup will catch it
					actualArgsSlice = append(actualArgsSlice, args)
					return iterator.FromSlice[string](nil), ""
				}
				actualArgsSlice = append(actualArgsSlice, args)
				ret := iterator.FromSlice(data[called])
				called++
				return ret, ""
			}, func(t *testing.T) {
				require.Equal(t, len(expectedArgs), called,
					"actual => %v", actualArgsSlice)
				for i, actualArgs := range actualArgsSlice {
					assert.Equal(t, expectedArgs[i], actualArgs, i)
				}
			}
	}
}

func nopDispatch() (func(string, ...string) bool, func(*testing.T)) {
	var called bool
	ret := func(command string, args ...string) bool {
		called = true
		return true
	}
	return ret, func(t *testing.T) {
		assert.False(t, called)
	}
}

func expectDispatch(expectedCmd string, expectedArgs ...string) func() (func(string, ...string) bool, func(*testing.T)) {
	return func() (func(string, ...string) bool, func(*testing.T)) {
		var called bool
		var actualCmd string
		var actualArgs []string
		ret := func(command string, args ...string) bool {
			called = true
			actualCmd = command
			actualArgs = args
			return true
		}
		return ret, func(t *testing.T) {
			assert.True(t, called)
			assert.Equal(t, expectedCmd, actualCmd)
			assert.Equal(t, append([]string{}, expectedArgs...), append([]string{}, actualArgs...))
		}
	}
}

func testNoManualCommands(cmds []string) (ret []text.CommandManual) {
	for _, cmd := range cmds {
		ret = append(ret, text.CommandManual{CommandManual: textapi.CommandManual{Name: cmd}})
	}
	return
}
