package cmd

import (
	"fmt"
	"os"
)

// serverTimeHint annotates the products delta-sync cursor.
const serverTimeHint = " (use as --updated-since for next delta sync)"

// emitServerTime prints the X-Server-Time value a response carried. In table
// mode it goes to stdout — the stream agents actually read — while json and
// quiet modes keep stdout machine-parseable and use stderr instead. No-op
// when the response carried no server time.
func emitServerTime(serverTime, hint string) {
	if serverTime == "" {
		return
	}
	w := os.Stderr
	if outputMode == "table" || outputMode == "" {
		w = os.Stdout
	}
	_, _ = fmt.Fprintf(w, "Server time: %s%s\n", serverTime, hint)
}
