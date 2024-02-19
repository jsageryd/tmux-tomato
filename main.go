package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
		duration: 25 * time.Minute,
		color:    203,
		icon:     "▘",
	},
	{
		duration: 5 * time.Minute,
		color:    191,
		icon:     "▖",
	},
	{
		duration: 25 * time.Minute,
		color:    203,
		icon:     "▘",
	},
	{
		duration: 5 * time.Minute,
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

	if timeLeft, active := eggTimer(now); active {
		if len(os.Args) < 2 {
			fmt.Printf("#[fg=color%d,bg=default]%s %s#[default]", eggTimerColor, timeLeft, eggTimerIcon)
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

	fmt.Printf("#[fg=color%d,bg=default]%s %s %s#[default]", curState.color, timeLeft, progressStr, curState.icon)
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

	stateFile := filepath.Join(home, ".tmux-tomato")

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
			}
		}
	}

	if eggTimerActive(ts) {
		return max(0, ts.Duration-now.Sub(ts.Start).Truncate(1*time.Second)), true
	}

	return 0, false
}
