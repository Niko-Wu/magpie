//go:build !darwin

package agent

import (
	"os"
	"strconv"
	"strings"

	"github.com/yetone/magpie/internal/proc"
)

// listRunning lists the processes a pgrep -f pattern matches, with how
// long each has run and its command line, from ps. Only the caller's own
// processes are listed: on a multi-user Linux server another user's Codex
// would otherwise be counted as a stale copy of this home's (#4240).
func listRunning(pat string) []runningProc {
	out, _ := proc.Command("pgrep", "-u", strconv.Itoa(os.Getuid()), "-f", pat).Output()
	pids := strings.Fields(string(out))
	if len(pids) == 0 {
		return nil
	}
	out, _ = proc.Command("ps", "-o", "etime=,command=", "-p", strings.Join(pids, ",")).Output()
	var ps []runningProc
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		et, cmd, _ := strings.Cut(strings.TrimSpace(l), " ")
		if up, ok := elapsed(et); ok {
			ps = append(ps, runningProc{up, strings.TrimSpace(cmd)})
		}
	}
	return ps
}
