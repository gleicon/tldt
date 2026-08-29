package detector

// raceEnabled is set by race_on.go (build tag: race) when the -race detector is
// active. Wall-clock budget tests skip when it is true, since race instrumentation
// adds roughly 60x overhead and makes any latency assertion meaningless.
var raceEnabled = false
