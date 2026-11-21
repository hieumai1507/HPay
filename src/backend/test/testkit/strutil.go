package testkit

import ()

func Contains(s, sub string) bool { return len(sub) > 0 && indexOf(s, sub) >= 0 }

func ContainsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !Contains(s, sub) {
			return false
		}
	}
	return true
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
