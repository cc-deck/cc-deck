package voice

import (
	"sync"
	"testing"
	"time"
)

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

func TestVAD_MinSpeechDuration(t *testing.T) {
	cfg := VADConfig{
		Threshold:            0.01,
		PreRollDuration:      0,
		SilenceDuration:      0.1,
		MaxUtteranceDuration: 5,
		MinSpeechDuration:    0.2,
	}
	vad := NewVAD(&cfg, 1000)

	frames := make(chan []int16, 10)
	go feedFrames(frames,
		makeSpeech(50, 5000),
		makeSilence(200),
	)

	utterances := collectUtterances(vad.Process(frames))
	if len(utterances) != 0 {
		t.Fatalf("got %d utterances from 50ms spike, want 0 (below 200ms min)", len(utterances))
	}
}

func TestVAD_MinSpeechDuration_PassesLong(t *testing.T) {
	cfg := VADConfig{
		Threshold:            0.01,
		PreRollDuration:      0,
		SilenceDuration:      0.1,
		MaxUtteranceDuration: 5,
		MinSpeechDuration:    0.2,
	}
	vad := NewVAD(&cfg, 1000)

	frames := make(chan []int16, 10)
	go feedFrames(frames,
		makeSpeech(300, 5000),
		makeSilence(200),
	)

	utterances := collectUtterances(vad.Process(frames))
	if len(utterances) != 1 {
		t.Fatalf("got %d utterances from 300ms speech, want 1 (above 200ms min)", len(utterances))
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

func TestVAD_HangoverKeepsTrailingAudio(t *testing.T) {
	cfg := VADConfig{
		Threshold:            0.01,
		PreRollDuration:      0,
		SilenceDuration:      0.2,
		HangoverDuration:     0.1,
		MaxUtteranceDuration: 5,
	}
	vad := NewVAD(&cfg, 1000)

	// 200 samples speech, then 300 samples silence (triggers at 200 silence samples).
	// With 100-sample hangover, the utterance should keep 100 samples of
	// trailing "silence" (which may contain low-energy speech in practice).
	frames := make(chan []int16, 10)
	go feedFrames(frames,
		makeSpeech(200, 5000),
		makeSilence(300),
	)

	utterances := collectUtterances(vad.Process(frames))
	if len(utterances) != 1 {
		t.Fatalf("got %d utterances, want 1", len(utterances))
	}
	// Without hangover: 200 samples (speech only, silence trimmed).
	// With hangover of 0.1s at 1000Hz = 100 samples kept: 200 + 100 = 300.
	if len(utterances[0].Audio) <= 200 {
		t.Errorf("utterance has %d samples, expected >200 (hangover should keep trailing audio)", len(utterances[0].Audio))
	}
}

func TestVAD_ZeroHangoverTrimsAll(t *testing.T) {
	cfg := VADConfig{
		Threshold:            0.01,
		PreRollDuration:      0,
		SilenceDuration:      0.2,
		HangoverDuration:     0,
		MaxUtteranceDuration: 5,
	}
	vad := NewVAD(&cfg, 1000)

	frames := make(chan []int16, 10)
	go feedFrames(frames,
		makeSpeech(200, 5000),
		makeSilence(300),
	)

	utterances := collectUtterances(vad.Process(frames))
	if len(utterances) != 1 {
		t.Fatalf("got %d utterances, want 1", len(utterances))
	}
	// Zero hangover trims all trailing silence, so only speech remains.
	if len(utterances[0].Audio) != 200 {
		t.Errorf("utterance has %d samples, expected 200 (zero hangover should trim all silence)", len(utterances[0].Audio))
	}
}

func TestUtteranceDuration(t *testing.T) {
	u := Utterance{Audio: make([]int16, 16000), SampleRate: 16000}
	d := UtteranceDuration(u)
	if d.Seconds() < 0.99 || d.Seconds() > 1.01 {
		t.Errorf("duration = %v, want ~1s", d)
	}
}

func TestVAD_TimingTwoUtterances(t *testing.T) {
	cfg := VADConfig{
		Threshold:            0.01,
		PreRollDuration:      0,
		SilenceDuration:      0.1,
		MaxUtteranceDuration: 5,
	}
	vad := NewVAD(&cfg, 1000)

	frames := make(chan []int16, 20)
	go feedFrames(frames,
		makeSpeech(200, 5000),  // 0-200ms speech
		makeSilence(200),       // 200-400ms silence (triggers end at 300ms = 200+100)
		makeSpeech(200, 5000),  // 400-600ms speech
		makeSilence(200),       // 600-800ms silence
	)

	utterances := collectUtterances(vad.Process(frames))
	if len(utterances) != 2 {
		t.Fatalf("got %d utterances, want 2", len(utterances))
	}

	// First utterance starts at 0 (no pre-roll, first speech).
	if utterances[0].Start != 0 {
		t.Errorf("u0.Start = %v, want 0", utterances[0].Start)
	}
	// End - Start equals the emitted audio duration.
	u0Dur := samplesToDuration(len(utterances[0].Audio), 1000)
	if utterances[0].End-utterances[0].Start != u0Dur {
		t.Errorf("u0 End-Start = %v, want %v", utterances[0].End-utterances[0].Start, u0Dur)
	}

	// Second utterance starts after the first. Non-overlapping.
	if utterances[1].Start < utterances[0].End {
		t.Errorf("u1.Start %v should be >= u0.End %v (non-overlapping)", utterances[1].Start, utterances[0].End)
	}
	// Increasing.
	if utterances[1].Start <= utterances[0].Start {
		t.Errorf("u1.Start %v should be > u0.Start %v (increasing)", utterances[1].Start, utterances[0].Start)
	}
	// End - Start equals the emitted audio duration.
	u1Dur := samplesToDuration(len(utterances[1].Audio), 1000)
	if utterances[1].End-utterances[1].Start != u1Dur {
		t.Errorf("u1 End-Start = %v, want %v", utterances[1].End-utterances[1].Start, u1Dur)
	}
}

func TestVAD_TimingPreRollIncluded(t *testing.T) {
	cfg := VADConfig{
		Threshold:            0.01,
		PreRollDuration:      0.1, // 100 samples at 1000 Hz
		SilenceDuration:      0.1,
		MaxUtteranceDuration: 5,
	}
	vad := NewVAD(&cfg, 1000)

	frames := make(chan []int16, 10)
	go feedFrames(frames,
		makeSilence(200),       // 200 samples silence (pre-roll captures last 100)
		makeSpeech(200, 5000),  // 200 samples speech
		makeSilence(200),       // 200 samples silence (ends utterance)
	)

	utterances := collectUtterances(vad.Process(frames))
	if len(utterances) != 1 {
		t.Fatalf("got %d utterances, want 1", len(utterances))
	}

	// With pre-roll of 100 samples, Start should be at 100ms (200ms - 100ms pre-roll).
	// The onset is at sample 200 (after 200 samples of silence), pre-roll captures
	// 100 samples before that, so Start should be at sample 100 = 100ms.
	wantStart := samplesToDuration(100, 1000)
	if utterances[0].Start != wantStart {
		t.Errorf("Start = %v, want %v (pre-roll should move Start back)", utterances[0].Start, wantStart)
	}

	// End - Start should equal the emitted audio duration.
	u0Dur := samplesToDuration(len(utterances[0].Audio), 1000)
	if utterances[0].End-utterances[0].Start != u0Dur {
		t.Errorf("End-Start = %v, want %v (should equal len(Audio)/SampleRate)", utterances[0].End-utterances[0].Start, u0Dur)
	}

	// The utterance includes pre-roll, so it has more samples than just the speech.
	if len(utterances[0].Audio) <= 200 {
		t.Errorf("Audio has %d samples, expected >200 (should include pre-roll)", len(utterances[0].Audio))
	}
}

func TestVAD_DynamicSilenceDurationChange(t *testing.T) {
	// Use a frame counter in the config function to change the silence
	// duration at a deterministic point. The first 3 frames use a long
	// silence (1s) so the 500-sample gap does not split them; from
	// frame 4 onward, silence drops to 0.1s so 200 samples of silence
	// ends the utterance.
	var mu sync.Mutex
	frameCount := 0
	vad := NewVADFunc(func() VADConfig {
		mu.Lock()
		frameCount++
		n := frameCount
		mu.Unlock()
		silDur := 1.0
		if n > 3 {
			silDur = 0.1
		}
		return VADConfig{
			Threshold:            0.01,
			PreRollDuration:      0,
			SilenceDuration:      silDur,
			MaxUtteranceDuration: 5,
		}
	}, 1000)

	frames := make(chan []int16, 30)
	go func() {
		frames <- makeSpeech(200, 5000)  // frame 1: speech
		frames <- makeSilence(500)       // frame 2: 500ms silence (< 1s, no split)
		frames <- makeSpeech(200, 5000)  // frame 3: more speech
		frames <- makeSilence(200)       // frame 4+: silence dur now 0.1s, 200ms >= 100ms
		close(frames)
	}()

	utterances := collectUtterances(vad.Process(frames))
	if len(utterances) != 1 {
		t.Fatalf("got %d utterances, want 1 (dynamic silence change)", len(utterances))
	}
}

func TestVAD_DynamicMaxSplitsContinuousSpeech(t *testing.T) {
	// A 12-second maximum (at 100 Hz for speed) should split continuous speech.
	vad := NewVADFunc(func() VADConfig {
		return VADConfig{
			Threshold:            0.01,
			PreRollDuration:      0,
			SilenceDuration:      100, // very long so only max triggers
			MaxUtteranceDuration: 12,
		}
	}, 100) // 100 Hz: 1 sample = 10ms, 12s = 1200 samples

	frames := make(chan []int16, 50)
	go func() {
		// Send 30 seconds of speech in 100-sample frames (1s each at 100 Hz).
		for i := 0; i < 30; i++ {
			frames <- makeSpeech(100, 5000)
		}
		// Silence to flush the final utterance.
		for i := 0; i < 200; i++ {
			frames <- makeSilence(100)
		}
		close(frames)
	}()

	utterances := collectUtterances(vad.Process(frames))
	// 30s of speech with 12s max should produce at least 2 utterances.
	if len(utterances) < 2 {
		t.Fatalf("got %d utterances from 30s speech with 12s max, want >= 2", len(utterances))
	}
}

func TestVAD_TimingFlushAtClose(t *testing.T) {
	cfg := VADConfig{
		Threshold:            0.01,
		PreRollDuration:      0,
		SilenceDuration:      1.0,
		MaxUtteranceDuration: 5,
	}
	vad := NewVAD(&cfg, 1000)

	frames := make(chan []int16, 5)
	go feedFrames(frames,
		makeSilence(100),       // 100 samples silence
		makeSpeech(300, 5000),  // 300 samples speech, then channel closes
	)

	utterances := collectUtterances(vad.Process(frames))
	if len(utterances) != 1 {
		t.Fatalf("got %d utterances, want 1 (flush at close)", len(utterances))
	}

	// Start should be at sample 100 (after 100 silence samples, no pre-roll).
	wantStart := samplesToDuration(100, 1000)
	if utterances[0].Start != wantStart {
		t.Errorf("Start = %v, want %v", utterances[0].Start, wantStart)
	}

	u0Dur := samplesToDuration(len(utterances[0].Audio), 1000)
	if utterances[0].End-utterances[0].Start != u0Dur {
		t.Errorf("End-Start = %v, want %v", utterances[0].End-utterances[0].Start, u0Dur)
	}
}

// TestVAD_SpeechBoundsExcludePadding verifies that SpeechStart and SpeechEnd
// mark the first and last loud audio of an utterance, independent of the
// pre-roll and hangover padding included in Start and End. The relay measures
// the pause between passages with these bounds.
func TestVAD_SpeechBoundsExcludePadding(t *testing.T) {
	cfg := VADConfig{
		Threshold:            0.01,
		PreRollDuration:      0.1,  // 100 samples at 1000 Hz
		SilenceDuration:      0.1,  // 100 samples
		HangoverDuration:     0.05, // 50 samples
		MaxUtteranceDuration: 5,
	}
	vad := NewVAD(&cfg, 1000)

	var input [][]int16
	add := func(n int, frame func() []int16) {
		for i := 0; i < n; i++ {
			input = append(input, frame())
		}
	}
	silence := func() []int16 { return makeSilence(50) }
	speech := func() []int16 { return makeSpeech(50, 5000) }
	add(4, silence) // 0-200 ms
	add(4, speech)  // 200-400 ms
	add(6, silence) // 400-700 ms (real pause: 300 ms)
	add(4, speech)  // 700-900 ms
	add(6, silence) // 900-1200 ms

	frames := make(chan []int16, len(input))
	go feedFrames(frames, input...)

	utterances := collectUtterances(vad.Process(frames))
	if len(utterances) != 2 {
		t.Fatalf("got %d utterances, want 2", len(utterances))
	}
	u0, u1 := utterances[0], utterances[1]

	ms := func(n int) time.Duration { return time.Duration(n) * time.Millisecond }
	if u0.SpeechStart != ms(200) || u0.SpeechEnd != ms(400) {
		t.Errorf("u0 speech bounds = [%v, %v], want [200ms, 400ms]", u0.SpeechStart, u0.SpeechEnd)
	}
	if u1.SpeechStart != ms(700) || u1.SpeechEnd != ms(900) {
		t.Errorf("u1 speech bounds = [%v, %v], want [700ms, 900ms]", u1.SpeechStart, u1.SpeechEnd)
	}
	if got := u1.SpeechStart - u0.SpeechEnd; got != ms(300) {
		t.Errorf("real pause = %v, want 300ms", got)
	}
	// Padding still exists on the audio bounds.
	if u0.Start >= u0.SpeechStart {
		t.Errorf("u0.Start %v should include pre-roll before SpeechStart %v", u0.Start, u0.SpeechStart)
	}
	if u0.End <= u0.SpeechEnd {
		t.Errorf("u0.End %v should include hangover after SpeechEnd %v", u0.End, u0.SpeechEnd)
	}
}
