package main

import (
	"fmt"
	"net/url"
	"strings"
)

// seedEnvs are the environments the seeder may run in. The seeder creates users
// and admins with known passwords, so production and unknown values are refused.
var seedEnvs = map[string]bool{"dev": true, "ci": true, "demo": true}

// localHosts are database hosts considered local: loopback, the compose service
// name, and Docker Desktop's host alias.
var localHosts = map[string]bool{
	"localhost": true, "127.0.0.1": true, "::1": true,
	"postgres": true, "host.docker.internal": true,
}

// checkSeedTarget decides whether the seeder may touch databaseURL under APP_ENV
// env. It returns the host it will touch. A non-local host needs allowRemote
// (the --allow-remote-db flag). The error names the host but never the password.
func checkSeedTarget(env, databaseURL string, allowRemote bool) (string, error) {
	if !seedEnvs[env] {
		return "", fmt.Errorf("refusing to seed: APP_ENV=%q (allowed: dev, ci, demo)", env)
	}
	if strings.TrimSpace(databaseURL) == "" {
		return "", fmt.Errorf("DATABASE_URL is required")
	}
	u, err := url.Parse(databaseURL)
	if err != nil || u.Hostname() == "" {
		return "", fmt.Errorf("DATABASE_URL is not a valid URL (host could not be determined)")
	}
	host := strings.ToLower(u.Hostname())
	if !localHosts[host] && !allowRemote {
		return host, fmt.Errorf("refusing to seed a non-local database host %q (the seeder creates accounts with known passwords); pass --allow-remote-db only if you are sure", host)
	}
	return host, nil
}
