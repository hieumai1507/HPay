package auth

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TC-RBA-007: the Go role table and the admin app's table must be identical.
func TestRolePermissionsMatchFrontend(t *testing.T) {
	// Local runs find the file by relative path; the Docker runner mounts it and sets
	// ADMIN_STORE_FILE. A missing file FAILS the test: a skipped drift check protects nothing.
	path := os.Getenv("ADMIN_STORE_FILE")
	if path == "" {
		path = "../../../admin/src/stores/admin.ts"
	}
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read the admin app role table (%s): %v", path, err)
	}
	text := string(src)
	start := strings.Index(text, "export const ROLE_PERMISSIONS")
	if start < 0 {
		t.Fatal("ROLE_PERMISSIONS not found in src/admin/src/stores/admin.ts")
	}
	block := text[start:]
	end := strings.Index(block, "};")
	if end < 0 {
		t.Fatal("end of ROLE_PERMISSIONS not found")
	}
	block = block[:end]

	front := map[string][]string{}
	roleRe := regexp.MustCompile(`(?s)(\w+):\s*\[(.*?)\]`)
	permRe := regexp.MustCompile(`'([a-z]+\.[a-z]+)'`)
	for _, m := range roleRe.FindAllStringSubmatch(block, -1) {
		var perms []string
		for _, p := range permRe.FindAllStringSubmatch(m[2], -1) {
			perms = append(perms, p[1])
		}
		sort.Strings(perms)
		front[m[1]] = perms
	}
	if len(front) != len(RolePermissions) {
		t.Fatalf("frontend has %d roles, backend %d", len(front), len(RolePermissions))
	}
	for role, ps := range RolePermissions {
		var back []string
		for _, p := range ps {
			back = append(back, string(p))
		}
		sort.Strings(back)
		if strings.Join(back, ",") != strings.Join(front[role], ",") {
			t.Errorf("role %s differs: backend %v, frontend %v", role, back, front[role])
		}
	}
}

func TestRoleHas(t *testing.T) {
	if RoleHas("viewer", PermKYCReview) || RoleHasAnyPermission("viewer") {
		t.Error("a viewer must hold nothing")
	}
	if !RoleHas("kyc_reviewer", PermKYCReview) || RoleHas("kyc_reviewer", PermReconRun) {
		t.Error("kyc_reviewer holds exactly kyc.review")
	}
	if RoleHas("nonexistent", PermKYCReview) || RoleHasAnyPermission("nonexistent") {
		t.Error("an unknown role must hold nothing")
	}
	if !RoleHasAny("ops", PermReconRun, PermAIManage) || RoleHasAny("ops", PermAIManage, PermOrgsEdit) {
		t.Error("RoleHasAny wrong for ops")
	}
}
