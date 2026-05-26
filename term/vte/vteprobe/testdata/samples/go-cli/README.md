// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

# go-cli sample

Reproduces a vteprobe `Infer` failure observed when navigating
`blue/cli/cli.go` in neovim with byoe.

Repro recipe captured in screenshots:

1. Open the file at line 51, column 1.
2. Press `k` once to move the cursor up to line 50.
3. At this point `Infer` returns a wrong result (overlay highlights
   slip out of alignment).
4. Press `h` (no-op horizontal motion). The next `Infer` succeeds and
   the overlay snaps back into place — proving the bug is a stale or
   mis-aligned probe, not a missing event.

The capture below should be taken at step 3 (the failed state).

## Refresh the capture

```bash
# from term/vte/vteprobe/testdata
./capture.sh go-cli nvim 50 1
```

The default `capture.sh` only opens the file at the (line, col)
given. To exercise the `k`-after-`j` motion that triggers the bug,
drive nvim manually with the same `tmux send-keys` flow capture.sh
uses, or temporarily extend capture.sh to accept a key sequence and
replay it before the capture-pane call:

```bash
tmux send-keys -t "$session" "51G1|" Enter   # land on line 51
tmux send-keys -t "$session" "k"             # move up to line 50 — triggers the bug
sleep 0.2
tmux capture-pane -t "$session" -p -e -S - >screen.ansi
tmux display-message -t "$session" -p '#{cursor_x},#{cursor_y}' >cursor.txt
```

`want.json` declares the ground-truth `Infer` should return for this
capture (file line 50, column 1 → `cursorAtScroll: {x:0, y:49}`).
The bug is reproduced by the test failing this assertion until the
probe is fixed.
