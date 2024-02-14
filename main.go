package main

import (
	"fmt"
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
}

func main() {
	now := time.Now()

	timeSinceMidnight := now.Sub(
		time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local),
	)

	var totalStateDuration time.Duration

	for _, s := range states {
		totalStateDuration += s.duration
	}

	progressInCurrentCycle := timeSinceMidnight % totalStateDuration

	var (
		curState state
		timeLeft time.Duration
	)

	var accStateDur time.Duration

	for _, s := range states {
		accStateDur += s.duration

		if progressInCurrentCycle <= accStateDur {
			curState = s
			timeLeft = accStateDur - progressInCurrentCycle
			break
		}
	}

	timeLeft = timeLeft.Truncate(time.Second)

	fmt.Printf("#[fg=color%d,bg=default]%s %s#[default]", curState.color, timeLeft, curState.icon)
}
