package service

import (
	"os/user"
	"sort"
	"strconv"

	"github.com/2panel-dev/2panel/internal/dto"
)

// allowedGIDs contains the primary group IDs whose members are eligible for
// task execution. GID 0 = root, 1000/1001 = typical first human users.
var allowedGIDs = []uint32{0, 1000, 1001}

// ListAccounts returns system users whose primary group or supplemental
// group membership matches one of the allowedGIDs.
func ListAccounts() (dto.AccountListResp, error) {
	allowedMap := make(map[uint32]bool, len(allowedGIDs))
	for _, g := range allowedGIDs {
		allowedMap[g] = true
	}

	cur, curErr := user.Current()

	accounts := make(map[string]bool)

	entries, err := parsePasswd()
	if err != nil {
		return dto.AccountListResp{}, err
	}

	for _, e := range entries {
		gid := parseUint32(e.GID)
		if allowedMap[gid] {
			accounts[e.Name] = true
		}
	}

	grpEntries, grpErr := parseGroup()
	if grpErr == nil {
		for _, ge := range grpEntries {
			gid := parseUint32(ge.GID)
			if !allowedMap[gid] {
				continue
			}
			for _, m := range ge.Members {
				accounts[m] = true
			}
		}
	}

	result := make([]string, 0, len(accounts))
	for name := range accounts {
		result = append(result, name)
	}
	sort.Strings(result)

	defaultUser := ""
	if curErr == nil {
		defaultUser = cur.Username
	}

	return dto.AccountListResp{
		Data:    result,
		Default: defaultUser,
		POSIX:   true,
	}, nil
}

func parseUint32(s string) uint32 {
	v, _ := strconv.ParseUint(s, 10, 32)
	return uint32(v)
}

type passwdEntry struct {
	Name string
	GID  string
}

type groupEntry struct {
	GID     string
	Members []string
}
