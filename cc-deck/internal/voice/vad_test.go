package voice

import "testing"

func makeSilence(n int) []int16 {
	return make([]int16, n)
}

func makeSpeech(n int, amplitude int16) []int16 {
	out := make([]int16, n)
	for i := range out {
		if i%2 == 0 {
			out[i] = amplitude
		} else {
			out[i] = -amplitude
		}
	}
	return out
}

func feedFrames(ch chan<- []int16, frames ...[]int16) {
	for _, f := range frames {
		ch <- f
	}
	close(ch)
}

func collectUtterances(ch <-chan Utterance) []Utterance {
	var result []Utterance
	for u := range ch {
		result = append(result, u)
	}
	return result
}

func TestVAD_SingleUtterance(t *testing.T) {
	cfg := VADConfig{
		Threshold:            0.01,
		PreRollDuration:      0,
		SilenceDuration:      0.1,
		MaxUtteranceDuration: 5,
	}
	vad := NewVAD(&cfg, 1000)

	frames := make(chan []int16, 10)
	go feedFrames(frames,
		makeSpeech(200, 5000),
		makeSilence(200),
	)

	utterances := collectUtterances(vad.Process(frames))
	if len(utterances) != 1 {
		t.Fatalf("got %d utterances, want 1", len(utterances))
	}
	if len(utterances[0].Audio) == 0 {
		t.Fatal("utterance has no audio")
	}
}

func TestVAD_TwoUtterances(t *testing.T) {
	cfg := VADConfig{
		Threshold:            0.01,
		PreRollDuration:      0,
		SilenceDuration:      0.1,
		MaxUtteranceDuration: 5,
	}
	vad := NewVAD(&cfg, 1000)

	frames := make(chan []int16, 20)
	go feedFrames(frames,
		makeSpeech(200, 5000),
		makeSilence(200),
		makeSpeech(200, 5000),
		makeSilence(200),
	)

	utterances := collectUtterances(vad.Process(frames))
	if len(utterances) != 2 {
		t.Fatalf("got %d utterances, want 2", len(utterances))
	}
}

func TestVAD_SilenceOnly(t *testing.T) {
	cfg := VADConfig{
		Threshold:            0.01,
		PreRollDuration:      0,
		SilenceDuration:      0.1,
		MaxUtteranceDuration: 5,
	}
	vad := NewVAD(&cfg, 1000)

	frames := make(chan []int16, 5)
	go feedFrames(frames,
		makeSilence(500),
		makeSilence(500),
	)

	utterances := collectUtterances(vad.Process(frames))
	if len(utterances) != 0 {
		t.Fatalf("got %d utterances from silence, want 0", len(utterances))
	}
}

func TestVAD_MaxDuration(t *testing.T) {
	cfg := VADConfig{
		Threshold:            0.01,
		PreRollDuration:      0,
		SilenceDuration:      0.5,
		MaxUtteranceDuration: 0.5,
	}
	vad := NewVAD(&cfg, 1000)

	// Send speech in small frames so max duration triggers between frames
	frames := make(chan []int16, 20)
	go func() {
		for i := 0; i < 10; i++ {
			frames <- makeSpeech(100, 5000)
		}
		for i := 0; i < 6; i++ {
			frames <- makeSilence(100)
		}
		close(frames)
	}()

	utterances := collectUtterances(vad.Process(frames))
	if len(utterances) < 1 {
		t.Fatal("expected at least 1 utterance from max-duration split")
	}
	for i, u := range utterances {
		if len(u.Audio) > 600 {
			t.Errorf("utterance %d has %d samples, expected <=600 (max 500 + one frame)", i, len(u.Audio))
		}
	}
}

func TestVAD_ChannelClosesMidUtterance(t *testing.T) {
	cfg := VADConfig{
		Threshold:            0.01,
		PreRollDuration:      0,
		SilenceDuration:      1.0,
		MaxUtteranceDuration: 5,
	}
	vad := NewVAD(&cfg, 1000)

	frames := make(chan []int16, 5)
	go feedFrames(frames,
		makeSpeech(200, 5000),
	)

	utterances := collectUtterances(vad.Process(frames))
	if len(utterances) != 1 {
		t.Fatalf("got %d utterances, want 1 (partial from channel close)", len(utterances))
	}
}

func TestVAD_PreRoll(t *testing.T) {
	cfg := VADConfig{
		Threshold:            0.01,
		PreRollDuration:      0.1,
		SilenceDuration:      0.1,
		MaxUtteranceDuration: 5,
	}
	vad := NewVAD(&cfg, 1000)

	frames := make(chan []int16, 10)
	go feedFrames(frames,
		makeSilence(200),
		makeSpeech(200, 5000),
		makeSilence(200),
	)

	utterances := collectUtterances(vad.Process(frames))
	if len(utterances) != 1 {
		t.Fatalf("got %d utterances, want 1", len(utterances))
	}
	if len(utterances[0].Audio) <= 200 {
		t.Errorf("utterance has %d samples, expected >200 (should include pre-roll)", len(utterances[0].Audio))
	}
}

func TestThresholdPercentRoundTrip(t *testing.T) {
	tests := []struct {
		rms     float64
		wantPct int
	}{
		{0.001, 0},
		{0.5, 100},
		{0.015, 44}, // default threshold lands near middle of usable range
	}
	for _, tt := range tests {
		pct := ThresholdToPercent(tt.rms)
		if pct != tt.wantPct {
			t.Errorf("ThresholdToPercent(%f) = %d, want %d", tt.rms, pct, tt.wantPct)
		}
	}

	for pct := 0; pct <= 100; pct += 5 {
		rms := PercentToThreshold(pct)
		got := ThresholdToPercent(rms)
		if got < pct-1 || got > pct+1 {
			t.Errorf("round-trip pct=%d -> rms=%f -> pct=%d (drift > 1)", pct, rms, got)
		}
	}
}

func TestThresholdPercentBoundaries(t *testing.T) {
	if ThresholdToPercent(0) != 0 {
		t.Error("below min should clamp to 0")
	}
	if ThresholdToPercent(1.0) != 100 {
		t.Error("above max should clamp to 100")
	}
	if PercentToThreshold(-10) != 0.001 {
		t.Error("negative percent should clamp to min RMS")
	}
	if PercentToThreshold(200) != 0.5 {
		t.Error("over 100 percent should clamp to max RMS")
	}
}

func TestRMSToLogScale(t *testing.T) {
	if got := RMSToLogScale(0.001); got != 0 {
		t.Errorf("RMSToLogScale(min) = %f, want 0", got)
	}
	if got := RMSToLogScale(0.5); got != 1 {
		t.Errorf("RMSToLogScale(max) = %f, want 1", got)
	}
	if got := RMSToLogScale(0); got != 0 {
		t.Errorf("RMSToLogScale(0) = %f, want 0", got)
	}
	mid := RMSToLogScale(0.015)
	if mid < 0.4 || mid > 0.5 {
		t.Errorf("RMSToLogScale(0.015) = %f, want ~0.44", mid)
	}
}

func TestUtteranceDuration(t *testing.T) {
	u := Utterance{Audio: make([]int16, 16000), SampleRate: 16000}
	d := UtteranceDuration(u)
	if d.Seconds() < 0.99 || d.Seconds() > 1.01 {
		t.Errorf("duration = %v, want ~1s", d)
	}
}
