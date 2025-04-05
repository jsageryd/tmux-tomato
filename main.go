package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type state struct {
	duration time.Duration
	color    int
	icon     string
}

// states is a list of states that the timer cycles through.
var states = []state{
	{
		duration: 50 * time.Minute,
		color:    203,
		icon:     "▘",
	},
	{
		duration: 10 * time.Minute,
		color:    191,
		icon:     "▖",
	},
}

const (
	eggTimerColor = 39
	eggTimerIcon  = "▌"
)

type eggTimerState struct {
	Start    time.Time     `json:"start"`
	Duration time.Duration `json:"duration"`
}

func main() {
	now := time.Now()

	var blockStr string

	blockFGColor := fmt.Sprintf("color%d", 255)
	blockBGColor := "default"

	nextBlockFGColor := fmt.Sprintf("color%d", 250)
	nextBlockBGColor := "default"

	{
		hhmmString := func(d time.Duration) string {
			haveHours := int(d.Hours()) > 0
			haveMinutes := int(d.Minutes())%60 > 0
			haveSeconds := int(d.Seconds())%60 > 0

			switch {
			case haveHours && haveMinutes:
				return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
			case haveHours && !haveMinutes:
				return fmt.Sprintf("%dh", int(d.Hours()))
			case !haveHours && haveMinutes:
				return fmt.Sprintf("%dm", int(d.Minutes())%60)
			case !haveHours && !haveMinutes && haveSeconds:
				return fmt.Sprintf("%ds", int(d.Seconds())%60)
			default:
				return "0s"
			}
		}

		blocks, err := readBlocks(now)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			fmt.Printf("error reading blocks file: %v\n", err)
			os.Exit(1)
		}

		if len(blocks) > 0 {
			blocks = linearizeBlocks(blocks)

			midnight := time.Date(blocks[0].Start.Year(), blocks[0].Start.Month(), blocks[0].Start.Day(), 0, 0, 0, 0, blocks[0].Start.Location())
			offset := blocks[0].Start.Sub(midnight)

			now := now // make now local to avoid affecting now outside of this block

			// In case now crosses over to the next day, add a day to now so that it
			// matches the block start time in case a block starts on the next day.
			if now.Day() != now.Add(-offset).Day() && now.Before(blocks[0].Start) {
				now = now.AddDate(0, 0, 1)
			}

			var cur, next Block

			if curIdx := slices.IndexFunc(blocks, func(b Block) bool {
				return b.covers(now)
			}); curIdx > -1 {
				cur = blocks[curIdx]
			}

			var nextIsFirst bool

			if nextIdx := slices.IndexFunc(blocks, func(b Block) bool {
				return b.Start.After(now)
			}); nextIdx > -1 {
				nextIsFirst = nextIdx == 0
				next = blocks[nextIdx]
			}

			if next == (Block{}) && len(blocks) > 0 {
				nextIsFirst = true
				next = blocks[0]
				next.Start = next.Start.AddDate(0, 0, 1)
			}

			if cur != (Block{}) {
				blockStr = fmt.Sprintf(" #[fg=%s,bg=%s]%s %s", blockFGColor, blockBGColor, cur.Start.Format("15:04"), cur.Desc)

				if cur.Duration > 0 {
					remainingTime := cur.end().Sub(now)
					blockStr += fmt.Sprintf(" %s (%s left)", hhmmString(cur.Duration), hhmmString(remainingTime))
				}
			}

			if next != (Block{}) {
				triangle := "▶"

				if cur == (Block{}) || (cur != (Block{}) && next.Start.Sub(cur.end()) > 0) {
					if nextIsFirst {
						triangle = "□"
					} else {
						triangle = "▷"
					}
				}

				blockStr += fmt.Sprintf(" #[fg=%s,bg=%s]%s %s %s", nextBlockFGColor, nextBlockBGColor, triangle, next.Start.Format("15:04"), next.Desc)

				if next.Duration > 0 {
					blockStr += " " + hhmmString(next.Duration)
				}
			}
		}
	}

	if timeLeft, active := eggTimer(now); active {
		if len(os.Args) < 2 {
			fgColor := fmt.Sprintf("color%d", eggTimerColor)
			bgColor := "default"

			var blinkStr string

			if timeLeft < 30*time.Second {
				blinkStr = fmt.Sprintf("#[fg=%s,blink,bg=%s]██████ #[default]", fgColor, bgColor)
				bgColor = fgColor
				fgColor = "color0"
			}

			statusStr := fmt.Sprintf(" %s#[fg=%s,bg=%s] %s %s#[default]", blinkStr, fgColor, bgColor, timeLeft, eggTimerIcon)

			if blockStr != "" {
				statusStr = blockStr + " |" + statusStr[1:]
			}

			fmt.Println(statusStr)
		}
		return
	}

	timeSinceMidnight := now.Sub(
		time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local),
	)

	var totalStateDuration time.Duration

	for _, s := range states {
		totalStateDuration += s.duration
	}

	progressInCurrentCycle := timeSinceMidnight % totalStateDuration

	var (
		progress int
		curState state
		timeLeft time.Duration
	)

	var accStateDur time.Duration

	for n, s := range states {
		accStateDur += s.duration

		if progressInCurrentCycle <= accStateDur {
			progress = n + 1
			curState = s
			timeLeft = accStateDur - progressInCurrentCycle
			break
		}
	}

	timeLeft = timeLeft.Truncate(time.Second)
	progressStr := strings.Repeat("■", progress) + strings.Repeat("□", len(states)-progress)

	fgColor := fmt.Sprintf("color%d", curState.color)
	bgColor := "default"

	var blinkStr string

	if timeLeft < 30*time.Second {
		blinkStr = fmt.Sprintf("#[fg=%s,blink,bg=%s]██████ #[default]", fgColor, bgColor)
		bgColor = fgColor
		fgColor = "color0"
	}

	statusStr := fmt.Sprintf(" %s#[fg=%s,bg=%s] %s %s %s#[default]", blinkStr, fgColor, bgColor, timeLeft, progressStr, curState.icon)

	if blockStr != "" {
		statusStr = blockStr + " |" + statusStr[1:]
	}

	fmt.Println(statusStr)
}

type Block struct {
	Start    time.Time
	Desc     string
	Duration time.Duration
}

func (b Block) covers(t time.Time) bool {
	return t.Equal(b.Start) || (t.After(b.Start) && (b.Duration == 0 || t.Before(b.Start.Add(b.Duration))))
}

func (b Block) end() time.Time {
	return b.Start.Add(b.Duration)
}

func (b Block) String() string {
	return fmt.Sprintf("%s %s %s", b.Start.Format("15:04"), b.Desc, b.Duration)
}

func readBlocks(now time.Time) ([]Block, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("error finding home directory: %v\n", err)
		os.Exit(1)
	}

	blocksFile := filepath.Join(home, ".tmux-tomato", "blocks")

	data, err := os.ReadFile(blocksFile)
	if err != nil {
		return nil, err
	}

	var blocks []Block
	var lastStart time.Time
	var lineNo int

	for s := bufio.NewScanner(bytes.NewReader(data)); s.Scan(); {
		lineNo++

		line := s.Text()
		if line == "" {
			continue
		}

		firstSpace := strings.Index(line, " ")
		lastSpace := strings.LastIndex(line, " ")

		if firstSpace == -1 || lastSpace == -1 || firstSpace == lastSpace {
			return nil, fmt.Errorf(`line %d: invalid format, want e.g. "10:00 Coffee break 1h"`, lineNo)
		}

		startTimeStr := line[:firstSpace]
		description := line[firstSpace+1 : lastSpace]
		durationStr := line[lastSpace+1:]

		start, err := time.Parse("15:04", startTimeStr)
		if err != nil {
			return nil, fmt.Errorf("line %d: cannot parse %q as an HH:MM timestamp", lineNo, startTimeStr)
		}

		start = time.Date(
			now.Year(), now.Month(), now.Day(),
			start.Hour(), start.Minute(), 0, 0,
			now.Location(),
		)

		if start.Equal(lastStart) {
			return nil, fmt.Errorf("line %d: there is already a block starting at %s", lineNo, start.Format("15:04"))
		}

		if start.Before(lastStart) {
			start = start.AddDate(0, 0, 1)
		}

		lastStart = start

		duration, err := time.ParseDuration(durationStr)
		if err != nil {
			return nil, fmt.Errorf("line %d: cannot parse %q as a duration", lineNo, durationStr)
		}

		blocks = append(blocks, Block{
			Start:    start,
			Desc:     description,
			Duration: duration,
		})
	}

	if len(blocks) > 0 {
		start := blocks[0].Start
		end := blocks[0].end()
		for _, b := range blocks[1:] {
			if b.end().After(end) {
				end = b.end()
			}
		}

		if end.Sub(start) > 24*time.Hour {
			return nil, fmt.Errorf("blocks may not span more than 24 hours")
		}
	}

	return blocks, nil
}

func linearizeBlocks(blocks []Block) []Block {
	var linear []Block
	var stack []Block
	var last Block

	if len(blocks) == 0 {
		return nil
	}

	midnight := blocks[0].Start.Truncate(24 * time.Hour)
	offset := blocks[0].Start.Sub(midnight)

	for _, cur := range blocks {
		cur.Start = cur.Start.Add(-offset)

		for len(stack) > 0 {
			if last.end().After(cur.Start) {
				break
			}

			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]

			shift := last.end().Sub(top.Start)
			top.Start = top.Start.Add(shift)
			top.Duration -= shift

			if top.Duration <= 0 {
				continue
			}

			linear = append(linear, top)

			last = top
		}

		if last.end().After(cur.Start) {
			stack = append(stack, last)
		}

		if len(linear) > 0 {
			if cur.Start.Before(linear[len(linear)-1].end()) {
				diff := cur.Start.Sub(linear[len(linear)-1].Start)
				if diff <= 0 {
					linear = linear[:len(linear)-1]
				} else if linear[len(linear)-1].Duration > diff {
					linear[len(linear)-1].Duration = diff
				}
			}
		}

		linear = append(linear, cur)

		last = cur
	}

	for len(stack) > 0 {
		top := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		shift := last.end().Sub(top.Start)
		top.Start = top.Start.Add(shift)
		top.Duration -= shift

		if top.Duration <= 0 {
			continue
		}

		linear = append(linear, top)

		last = top
	}

	for n := range linear {
		linear[n].Start = linear[n].Start.Add(offset)
	}

	return linear
}

func eggTimer(now time.Time) (timeLeft time.Duration, active bool) {
	eggTimerActive := func(ts eggTimerState) bool {
		return ts.Duration > 0 && ts.Start.Add(ts.Duration+5*time.Second).After(now)
	}

	var ts eggTimerState

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("error finding home directory: %v\n", err)
		os.Exit(1)
	}

	stateFile := filepath.Join(home, ".tmux-tomato", "timer")

	if len(os.Args) == 2 {
		duration, err := time.ParseDuration(os.Args[1])
		if err != nil {
			fmt.Printf("Usage: tmux-tomato [duration]\nSpecify duration (e.g. 2h, 25m, 5m30s) to set egg timer.\n")
			os.Exit(1)
		}

		ts = eggTimerState{
			Start:    now,
			Duration: duration,
		}

		b, err := json.Marshal(ts)
		if err != nil {
			fmt.Printf("error marshalling egg timer state: %v\n", err)
			os.Exit(1)
		}

		if err := os.MkdirAll(filepath.Dir(stateFile), 0o777); err != nil {
			fmt.Printf("error creating directory %s: %v\n", filepath.Dir(stateFile), err)
			os.Exit(1)
		}

		if err := os.WriteFile(stateFile, b, 0o666); err != nil {
			fmt.Printf("error writing egg timer state: %v\n", err)
			os.Exit(1)
		}
	} else {
		if b, err := os.ReadFile(stateFile); err == nil {
			if err := json.Unmarshal(b, &ts); err != nil {
				fmt.Printf("error unmarshalling egg timer state: %v\n", err)
				os.Exit(1)
			}

			if !eggTimerActive(ts) {
				if err := os.Remove(stateFile); err != nil {
					if !os.IsNotExist(err) {
						fmt.Printf("error removing egg timer state file: %v\n", err)
						os.Exit(1)
					}
				}

				// Try to remove the directory as well in case it's empty. I found no
				// good cross-platform way to check for syscall.ENOTEMPTY which is what
				// is returned if the directory is not empty, so this just ignores all
				// errors instead. Good enough.
				os.Remove(filepath.Dir(stateFile))
			}
		}
	}

	if eggTimerActive(ts) {
		return max(0, ts.Duration-now.Sub(ts.Start).Truncate(1*time.Second)), true
	}

	return 0, false
}
