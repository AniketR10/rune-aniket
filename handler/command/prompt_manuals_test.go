package command

import (
	"testing"

	"github.com/ernestrc/blue/document"
	log "github.com/sirupsen/logrus"
	textapi "unstable.build/go-tui/api/text"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
	testutil "unstable.build/go-tui/util/test"
)

var goodTestCommands = []text.CommandManual{
	{CommandManual: textapi.CommandManual{Name: "subaru", Summary: "2021 top of the line, high tech.", Synopsis: "outback touring xt"}},
	{CommandManual: textapi.CommandManual{Name: "jeep", Summary: "2022 bottom of the line, great offroading.", Synopsis: "gladiator sport s"}},
	{CommandManual: textapi.CommandManual{Name: "mercedes", Summary: "2014 old luxury car.", Synopsis: "GL 450",
		Commands: []textapi.CommandManual{
			{Name: "GL", Summary: "GLs are 7 seater.", Synopsis: "[450]",
				Commands: []textapi.CommandManual{
					{Name: "450", Summary: "450 is middle tier", Synopsis: ""}},
			}},
	}},
	{CommandManual: textapi.CommandManual{Name: "gladiator"}, AliasOf: []string{"jeep"}},
	{CommandManual: textapi.CommandManual{Name: "current"}, AliasOf: []string{"subaru", "jeep"}},
}

var goodLotsTestCommands = []text.CommandManual{
	{CommandManual: textapi.CommandManual{Name: "0", Summary: "The void.", Synopsis: "<nothing>"}},
	{CommandManual: textapi.CommandManual{Name: "1", Summary: "Top of the line.", Synopsis: "<nothing>"}},
	{CommandManual: textapi.CommandManual{Name: "2", Summary: "Next in kin"}},
	{CommandManual: textapi.CommandManual{Name: "3", Summary: "Podium."}},
	{CommandManual: textapi.CommandManual{Name: "4", Summary: "Who knows."}},
	{CommandManual: textapi.CommandManual{Name: "5", Summary: "Who cares."}},
	{CommandManual: textapi.CommandManual{Name: "6", Summary: "Say what?"}},
	{CommandManual: textapi.CommandManual{Name: "7", Summary: "Cool."}},
	{CommandManual: textapi.CommandManual{Name: "8", Summary: "Numbers."}},
	{CommandManual: textapi.CommandManual{Name: "9", Summary: "Bottom of the line."}},
}

func TestCommandHandlerManualsDraw(t *testing.T) {
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
                                        
                                        
                                        
                                        
                                        
                                        
────────────────────────────────────────
                                        
USAGE                                   
subaru outback touring xt               
                                        
DESCRIPTION                             
2021 top of the line, high tech.        
                                        `},
		{"initializes with some commands search match", "e", goodTestCommands,
			`
e▐                                      
jeep                                    
mercedes                                
current                                 
                                        
                                        
                                        
                                        
                                        
                                        
                                        
────────────────────────────────────────
                                        
USAGE                                   
jeep gladiator sport s                  
                                        
DESCRIPTION                             
2022 bottom of the line, great          
offroading.                             
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
8                                       
9                                       
                                        
────────────────────────────────────────
                                        
USAGE                                   
0 <nothing>                             
                                        
DESCRIPTION                             
The void.                               
                                        `},

		{"command NOT in list with no args",
			"1", nil, `
1▐                                      
                                        
                                        
                                        
                                        
                                        
                                        
                                        
                                        
                                        
                                        
                                        
                                        
                                        
                                        
                                        
                                        
                                        
                                        
                                        `},
		{"fully typed command with args no subcommand delete in the middle",
			"merce ~^^^^^^^^^erce my", goodTestCommands, `
mercedes my▐                            
                                        
                                        
                                        
                                        
                                        
                                        
                                        
────────────────────────────────────────
                                        
USAGE                                   
mercedes GL 450                         
                                        
DESCRIPTION                             
2014 old luxury car.                    
                                        
SUB-COMMANDS                            
- GL                                    
                                        
                                        `},
		{"partially typed command with args no subcommand delete in the middle",
			"merce ~^^^^^^^^^erce my", goodTestCommands, `
mercedes my▐                            
                                        
                                        
                                        
                                        
                                        
                                        
                                        
────────────────────────────────────────
                                        
USAGE                                   
mercedes GL 450                         
                                        
DESCRIPTION                             
2014 old luxury car.                    
                                        
SUB-COMMANDS                            
- GL                                    
                                        
                                        `},
		{"fully typed command with space, completed via manual",
			"mercedes ", goodTestCommands, `
mercedes ▐                              
GL                                      
                                        
                                        
                                        
                                        
                                        
                                        
────────────────────────────────────────
                                        
USAGE                                   
mercedes GL 450                         
                                        
DESCRIPTION                             
2014 old luxury car.                    
                                        
SUB-COMMANDS                            
- GL                                    
                                        
                                        `},
		{"fully typed command with partially typed subcommand, completed via manual",
			"mercedes G", goodTestCommands, `
mercedes G▐                             
GL                                      
                                        
                                        
                                        
                                        
                                        
                                        
────────────────────────────────────────
                                        
USAGE                                   
mercedes GL 450                         
                                        
DESCRIPTION                             
2014 old luxury car.                    
                                        
SUB-COMMANDS                            
- GL                                    
                                        
                                        `},
		{"fully typed command with fully typed 1st subcommand, completed via manual",
			"mercedes GL", goodTestCommands, `
mercedes GL▐                            
GL                                      
                                        
                                        
                                        
                                        
                                        
                                        
────────────────────────────────────────
                                        
USAGE                                   
mercedes GL 450                         
                                        
DESCRIPTION                             
2014 old luxury car.                    
                                        
SUB-COMMANDS                            
- GL                                    
                                        
                                        `},
		{"fully typed command with fully typed 1st subcommand, with space, completed via manual",
			"mercedes GL ", goodTestCommands, `
mercedes GL ▐                           
450                                     
                                        
                                        
                                        
                                        
                                        
                                        
────────────────────────────────────────
                                        
USAGE                                   
GL [450]                                
                                        
DESCRIPTION                             
GLs are 7 seater.                       
                                        
SUB-COMMANDS                            
- 450                                   
                                        
                                        `},
		{"fully typed command with partially typed 2nd subcommand, completed via manual",
			"mercedes GL 4", goodTestCommands, `
mercedes GL 4▐                          
450                                     
                                        
                                        
                                        
                                        
                                        
                                        
────────────────────────────────────────
                                        
USAGE                                   
GL [450]                                
                                        
DESCRIPTION                             
GLs are 7 seater.                       
                                        
SUB-COMMANDS                            
- 450                                   
                                        
                                        `},
		{"fully typed command with fully typed 2nd subcommand, completed via manual",
			"mercedes GL 450", goodTestCommands, `
mercedes GL 450▐                        
450                                     
                                        
                                        
                                        
                                        
                                        
                                        
────────────────────────────────────────
                                        
USAGE                                   
GL [450]                                
                                        
DESCRIPTION                             
GLs are 7 seater.                       
                                        
SUB-COMMANDS                            
- 450                                   
                                        
                                        `},
		{"fully typed command with fully typed 2nd subcommand, with space, completed via manual",
			"mercedes GL 450 ", goodTestCommands, `
mercedes GL 450 ▐                       
                                        
                                        
                                        
                                        
                                        
                                        
                                        
                                        
                                        
                                        
                                        
────────────────────────────────────────
                                        
USAGE                                   
450                                     
                                        
DESCRIPTION                             
450 is middle tier                      
                                        `},
		{"fully typed command with fully typed 2nd subcommand, deleted last space, completed via manual",
			"mercedes GL 450 ^", goodTestCommands, `
mercedes GL 450▐                        
450                                     
                                        
                                        
                                        
                                        
                                        
                                        
────────────────────────────────────────
                                        
USAGE                                   
GL [450]                                
                                        
DESCRIPTION                             
GLs are 7 seater.                       
                                        
SUB-COMMANDS                            
- 450                                   
                                        
                                        `},
		{"fully typed command with fully typed 2nd subcommand, deleted space and , half arg, completed via manual",
			"mercedes GL 450 ^^^", goodTestCommands, `
mercedes GL 4▐                          
450                                     
                                        
                                        
                                        
                                        
                                        
                                        
────────────────────────────────────────
                                        
USAGE                                   
GL [450]                                
                                        
DESCRIPTION                             
GLs are 7 seater.                       
                                        
SUB-COMMANDS                            
- 450                                   
                                        
                                        `},
		{"fully typed command with fully typed 2nd subcommand, deleted last arg, completed via manual",
			"mercedes GL 450 ^^^^", goodTestCommands, `
mercedes GL ▐                           
450                                     
                                        
                                        
                                        
                                        
                                        
                                        
────────────────────────────────────────
                                        
USAGE                                   
GL [450]                                
                                        
DESCRIPTION                             
GLs are 7 seater.                       
                                        
SUB-COMMANDS                            
- 450                                   
                                        
                                        `},
		{"fully typed command with fully typed 2nd subcommand, deleted last arg, completed via manual",
			"mercedes GL 450 ^^^^^", goodTestCommands, `
mercedes GL▐                            
GL                                      
                                        
                                        
                                        
                                        
                                        
                                        
────────────────────────────────────────
                                        
USAGE                                   
mercedes GL 450                         
                                        
DESCRIPTION                             
2014 old luxury car.                    
                                        
SUB-COMMANDS                            
- GL                                    
                                        
                                        `},
		{"alias of multiple commands",
			"current", goodTestCommands, `
current▐                                
current                                 
                                        
                                        
                                        
                                        
────────────────────────────────────────
                                        
USAGE                                   
current                                 
                                        
DESCRIPTION                             
Alias of the following sequence of      
commands:                               
                                        
- subaru                                
- jeep                                  
                                        
                                        
                                        `},
		{"alias one command",
			"gladiator", goodTestCommands, `
gladiator▐                              
gladiator                               
                                        
                                        
                                        
                                        
                                        
                                        
                                        
                                        
                                        
────────────────────────────────────────
                                        
USAGE                                   
gladiator                               
                                        
DESCRIPTION                             
Alias of jeep                           
                                        
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
				{InputSequence: tcase.sequence, Expected: tcase.expectedDraw[1:]},
			}
			testutil.TestHandlerSequence(t, testCommandHandler{b}, 40, 20, cases)
		})
	}
}
