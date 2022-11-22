package main

import (
	"context"
	"strconv"
	"testing"

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/iterator"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
	testutil "unstable.build/go-tui/util/test"
)

func TestCommandHandlerDraw(t *testing.T) {
	storage := document.NewInMemoryService()
	commandKey := term.KeyComb{Ch: '@'}
	maxHistory := 100
	overlayCfg := text.DefaultCommandOverlayConfig()

	tsuite := []struct {
		desc         string
		sequence     string
		commands     []string
		completeCmd  func() (func(ctx context.Context, command string, args ...string) iterator.Iterator[string], func(*testing.T))
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
		{"dispatches command NOT in list with no args",
			"1>", []string{"lane", "lorelai", "rori"},
			nopComplete, expectDispatch("1"), `
▐                   
lane                
lorelai             
rori                
                    
                    
                    
                    
                    
                    `},
		{"draw command in list with no args",
			"lo", []string{"lane", "lorelai", "rori"},
			nopComplete, nopDispatch, `
lo▐                 
lorelai             
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"dispatches command in list with no args",
			"lo>", []string{"lane", "lorelai", "rori"},
			nopComplete, expectDispatch("lorelai"), `
▐                   
lane                
lorelai             
rori                
                    
                    
                    
                    
                    
                    `},
		{"draw command NOT in list with args no auto-complete",
			"1 /tmp/a", []string{"lane", "lorelai", "rori"},
			nopComplete, nopDispatch, `
1 /tmp/a▐           
                    
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"dispatches command NOT in list with args no auto-complete",
			"1 /tmp/a>", []string{"lane", "lorelai", "rori"},
			nopComplete, expectDispatch("1", "/tmp/a"), `
▐                   
lane                
lorelai             
rori                
                    
                    
                    
                    
                    
                    `},
		{"draw command in list with args no auto-complete",
			"lo /tmp/a", []string{"lane", "lorelai", "rori"},
			nopComplete, nopDispatch, `
lorelai /tmp/a▐     
                    
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"dispatches command with args no auto-complete",
			"lo /tmp/a>", []string{"lane", "lorelai", "rori"},
			nopComplete, expectDispatch("lorelai", "/tmp/a"), `
▐                   
lane                
lorelai             
rori                
                    
                    
                    
                    
                    
                    `},
		{"draw command with args with auto-complete",
			"rori my", []string{"lane", "lorelai", "rori"},
			completeWith("myArg"), nopDispatch, `
rori my▐            
myArg               
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"dispatches command with args with auto-complete",
			"rori my>", []string{"lane", "lorelai", "rori"},
			completeWith("myArg"), expectDispatch("rori", "myArg"), `
▐                   
lane                
lorelai             
rori                
                    
                    
                    
                    
                    
                    `},
		{"draw fully typed command with args with auto-complete",
			"rori my", []string{"lane", "lorelai", "rori"},
			completeWith("myArg"), nopDispatch, `
rori my▐            
myArg               
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"dispatches fully typed command with args with auto-complete",
			"rori my>", []string{"lane", "lorelai", "rori"},
			completeWith("myArg"), expectDispatch("rori", "myArg"), `
▐                   
lane                
lorelai             
rori                
                    
                    
                    
                    
                    
                    `},
		{"draw fully typed command with args with auto-complete and delete in the middle",
			"rori ^ my", []string{"lane", "lorelai", "rori"},
			completeWith("myArg"), nopDispatch, `
rori my▐            
myArg               
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"dispatches fully typed command with args with auto-complete and delete in the middle",
			"rori ^ my>", []string{"lane", "lorelai", "rori"},
			completeWith("myArg"), expectDispatch("rori", "myArg"), `
▐                   
lane                
lorelai             
rori                
                    
                    
                    
                    
                    
                    `},
		{"draw command with args with auto-complete one last space",
			"ro my ", []string{"lane", "lorelai", "rori"},
			completeRespectively([]string{"myArg"}), nopDispatch, `
rori myArg ▐        
                    
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"dispatches command with args with auto-complete one last space",
			"ro my >", []string{"lane", "lorelai", "rori"},
			completeRespectively([]string{"myArg"}), expectDispatch("rori", "myArg"), `
▐                   
lane                
lorelai             
rori                
                    
                    
                    
                    
                    
                    `},
		{"draw command with args with auto-complete one last space that's removed",
			"ro my ^", []string{"lane", "lorelai", "rori"},
			completeRespectively([]string{"myArg"}), nopDispatch, `
rori myArg▐         
myArg               
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"dispatches command with args with auto-complete one last space that's removed",
			"ro my ^>", []string{"lane", "lorelai", "rori"},
			completeRespectively([]string{"myArg"}), expectDispatch("rori", "myArg"), `
▐                   
lane                
lorelai             
rori                
                    
                    
                    
                    
                    
                    `},
		{"draw command with args with auto-complete delete and re-typed all",
			"ro my ^^^^^^^^^^^ro my a", []string{"lane", "lorelai", "rori"},
			completeRespectively([]string{"myArg"}), nopDispatch, `
rori myArg a▐       
                    
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"dispatches command with args with auto-complete delete and re-typed all",
			"ro my ^^^^^^^^^^^ro my a>", []string{"lane", "lorelai", "rori"},
			completeRespectively([]string{"myArg"}), expectDispatch("rori", "myArg", "a"), `
▐                   
lane                
lorelai             
rori                
                    
                    
                    
                    
                    
                    `},
		{"draw delete and re-type all with no autocomplete",
			"rori myArg ^^^^^^^^^^^rori myArg a", []string{"lane", "lorelai", "rori"},
			nopComplete, nopDispatch, `
rori myArg a▐       
                    
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"dispatch delete and re-type all with no autocomplete",
			"rori myArg ^^^^^^^^^^^rori myArg a>", []string{"lane", "lorelai", "rori"},
			nopComplete, expectDispatch("rori", "myArg", "a"), `
▐                   
lane                
lorelai             
rori                
                    
                    
                    
                    
                    
                    `},
		{"draw command from history no autocomplete",
			"rori myArg>lorelai myArg>@ oArg", []string{"lane", "lorelai", "rori"},
			nopComplete, expectDispatch("lorelai", "myArg"), `
lorelai myArg oArg▐ 
                    
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"dispatches command from history no autocomplete",
			"rori myArg>lorelai myArg>@ oArg>", []string{"lane", "lorelai", "rori"},
			nopComplete, expectDispatch("lorelai", "myArg", "oArg"), `
▐                   
lane                
lorelai             
rori                
                    
                    
                    
                    
                    
                    `},
		{"draw from history with autocomplete",
			"lo my>ro my>@", []string{"lane", "lorelai", "rori"},
			completeWith("myArg"), expectDispatch("rori", "myArg"), `
rori myArg▐         
myArg               
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"dispatch from history with autocomplete",
			"lo my>ro my>@ oArg>", []string{"lane", "lorelai", "rori"},
			completeWith("myArg"), expectDispatch("rori", "myArg", "oArg"), `
▐                   
lane                
lorelai             
rori                
                    
                    
                    
                    
                    
                    `},
		{"draw delete after load from history with autocomplete",
			"lo my>ro my>@^^^^^", []string{"lane", "lorelai", "rori"},
			completeWith("myArg"), expectDispatch("rori", "myArg"), `
rori ▐              
myArg               
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"dispatch delete after load from history with autocomplete",
			"lo my>ro my>@^^^^^oArg>", []string{"lane", "lorelai", "rori"},
			completeWith("myArg"), expectDispatch("rori", "oArg"), `
▐                   
lane                
lorelai             
rori                
                    
                    
                    
                    
                    
                    `},
		{"draw delete after load from history with autocomplete scroll through history",
			"lo my>ro my>@@^^^^^oArg", []string{"lane", "lorelai", "rori"},
			completeWith("myArg"), expectDispatch("rori", "myArg"), `
lorelai oArg▐       
                    
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"dispatch delete after load from history with autocomplete scroll through history",
			"lo my>ro my>@@^^^^^oArg>", []string{"lane", "lorelai", "rori"},
			completeWith("myArg"), expectDispatch("lorelai", "oArg"), `
▐                   
lane                
lorelai             
rori                
                    
                    
                    
                    
                    
                    `},
		{"draw command in list with extra spaces in args no auto-complete",
			"lo   /tmp/a", []string{"lane", "lorelai", "rori"},
			nopComplete, nopDispatch, `
lorelai   /tmp/a▐   
                    
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"dispatches command with extra spaces in args no auto-complete",
			"lo   /tmp/a>", []string{"lane", "lorelai", "rori"},
			nopComplete, expectDispatch("lorelai", "/tmp/a"), `
▐                   
lane                
lorelai             
rori                
                    
                    
                    
                    
                    
                    `},
		{"draw command in list with extra spaces in args that are deleted no auto-complete",
			"lo   ^^^ /tmp/a", []string{"lane", "lorelai", "rori"},
			nopComplete, nopDispatch, `
lorelai /tmp/a▐     
                    
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"dispatches command with extra spaces in args that are deleted no auto-complete",
			"lo   ^^/tmp/a>", []string{"lane", "lorelai", "rori"},
			nopComplete, expectDispatch("lorelai", "/tmp/a"), `
▐                   
lane                
lorelai             
rori                
                    
                    
                    
                    
                    
                    `},
		{"draw command with multiple args with auto-complete",
			"ro my ore", []string{"lane", "lorelai", "rori"},
			expectCompleteWith([][]string{{}, {"myArg"}}, [][]string{{"myArg"}, {"oregano", "oregani"}}),
			nopDispatch, `
rori myArg ore▐     
oregano             
oregani             
                    
                    
                    
                    
                    
                    
                    `},
		{"dispatches command with multiple args with auto-complete",
			"ro my ore>", []string{"lane", "lorelai", "rori"},
			expectCompleteWith([][]string{{}, {"myArg"}}, [][]string{{"myArg"}, {"oregano", "oregani"}}),
			expectDispatch("rori", "myArg", "oregano"), `
▐                   
lane                
lorelai             
rori                
                    
                    
                    
                    
                    
                    `},
		{"tab cycles through list if multiple results",
			"ro my ore✌>", []string{"lane", "lorelai", "rori"},
			expectCompleteWith([][]string{{}, {"myArg"}}, [][]string{{"myArg"}, {"oregano", "oregani"}}),
			expectDispatch("rori", "myArg", "oregani"), `
▐                   
lane                
lorelai             
rori                
                    
                    
                    
                    
                    
                    `},
		{"tab completes if there's only one match in the list",
			"ro my gani✌", []string{"lane", "lorelai", "rori"},
			expectCompleteWith([][]string{{}, {"myArg"}, {"myArg", "oregani"}},
				[][]string{{"myArg"}, {"oregano", "oregani"}}),
			nopDispatch, `
rori myArg oregani ▐
                    
                    
                    
                    
                    
                    
                    
                    
                    `},
	}

	for _, tcase := range tsuite {
		log.SetLevel(log.TraceLevel)
		tcase := tcase
		t.Run(tcase.desc, func(t *testing.T) {
			dispatchFn, cleanup := tcase.dispatchCmd()
			defer cleanup(t)

			completeFn, cleanupComplete := tcase.completeCmd()
			defer cleanupComplete(t)

			b := newCommandListHandler(
				storage, maxHistory, overlayCfg, commandKey,
				completeFn, dispatchFn, func() {},
			)
			b.dataReset(tcase.commands)
			defer b.Close()
			cases := []testutil.HandlerSequenceTestCase{
				{InputSequence: tcase.sequence, Expected: tcase.expectedDraw[1:]},
			}
			testutil.TestHandlerSequence(t, testCommandHandler{b}, 20, 10, cases)
		})
	}
}

type testCommandHandler struct {
	*commandListHandler
}

func (t testCommandHandler) Handle(ev term.Event) (bool, bool) {
	quit, handled := t.commandListHandler.Handle(ev)
	t.list.Wait()
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

func nopComplete() (func(context.Context, string, ...string) iterator.Iterator[string], func(*testing.T)) {
	return func(ctx context.Context, command string, args ...string) iterator.Iterator[string] {
		return iterator.FromSlice[string](nil)
	}, func(*testing.T) {}
}

func completeWith(data ...string) func() (func(context.Context, string, ...string) iterator.Iterator[string], func(*testing.T)) {
	return func() (func(ctx context.Context, command string, args ...string) iterator.Iterator[string], func(*testing.T)) {
		return func(ctx context.Context, command string, args ...string) iterator.Iterator[string] {
			return iterator.FromSlice(data)
		}, func(*testing.T) {}
	}
}

func completeRespectively(data []string) func() (func(context.Context, string, ...string) iterator.Iterator[string], func(*testing.T)) {
	return func() (func(context.Context, string, ...string) iterator.Iterator[string], func(*testing.T)) {
		return func(ctx context.Context, command string, args ...string) iterator.Iterator[string] {
			if len(args) >= len(data) {
				return iterator.FromSlice[string](nil)
			}
			completing := []string{data[len(args)]}
			return iterator.FromSlice(completing)
		}, func(*testing.T) {}
	}
}

func expectCompleteWith(expectedArgs [][]string, data [][]string) func() (func(ctx context.Context, command string, args ...string) iterator.Iterator[string], func(*testing.T)) {
	var actualArgsSlice [][]string
	var called int
	return func() (func(context.Context, string, ...string) iterator.Iterator[string], func(*testing.T)) {
		return func(ctx context.Context, command string, args ...string) iterator.Iterator[string] {
				if called >= len(data) {
					called++ // cleanup will catch it
					return iterator.FromSlice[string](nil)
				}
				actualArgsSlice = append(actualArgsSlice, args)
				ret := iterator.FromSlice(data[called])
				called++
				return ret
			}, func(t *testing.T) {
				require.Equal(t, len(expectedArgs), called)
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
			require.True(t, called)
			assert.Equal(t, expectedCmd, actualCmd)
			assert.Equal(t, append([]string{}, expectedArgs...), append([]string{}, actualArgs...))
		}
	}
}
